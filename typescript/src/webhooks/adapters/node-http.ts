import type { IncomingMessage, ServerResponse, RequestListener } from "node:http";
import { WebhookReceiver, WebhookVerificationError } from "../handler.js";

export interface NodeHandlerOptions {
  /** URL path to handle (default: "/webhooks/basecamp"). Requests to other paths get 404. */
  path?: string;
}

/**
 * Create a Node.js HTTP request listener that dispatches to a WebhookReceiver.
 * Returns 200 on success, 401 on bad signature, 405 on non-POST, 404 on wrong path.
 */
export function createNodeHandler(
  receiver: WebhookReceiver,
  options?: NodeHandlerOptions,
): RequestListener {
  const targetPath = options?.path ?? "/webhooks/basecamp";

  return (req: IncomingMessage, res: ServerResponse) => {
    // Parse URL path (handle both absolute and relative URLs)
    const url = req.url ?? "/";
    const pathname = url.split("?")[0];

    if (pathname !== targetPath) {
      res.writeHead(404);
      res.end("Not Found");
      return;
    }

    if (req.method !== "POST") {
      res.writeHead(405);
      res.end("Method Not Allowed");
      return;
    }

    const chunks: Buffer[] = [];
    req.on("data", (chunk: Buffer) => chunks.push(chunk));
    req.on("end", () => {
      const body = Buffer.concat(chunks);
      const headers = (name: string) => {
        const val = req.headers[name.toLowerCase()];
        return Array.isArray(val) ? val[0] : val;
      };

      receiver
        .handleRequest(body, headers)
        .then(() => {
          res.writeHead(200);
          res.end("OK");
        })
        .catch((err: unknown) => {
          if (err instanceof WebhookVerificationError) {
            res.writeHead(401);
            res.end("Unauthorized");
          } else {
            res.writeHead(500);
            res.end("Internal Server Error");
          }
        });
    });
  };
}
