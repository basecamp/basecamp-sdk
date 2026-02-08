import type { IncomingMessage, ServerResponse, RequestListener } from "node:http";
import { WebhookReceiver, WebhookVerificationError } from "../handler.js";

export interface NodeHandlerOptions {
  /** URL path to handle (default: "/webhooks/basecamp"). Requests to other paths get 404. */
  path?: string;
  /** Maximum request body size in bytes (default: 1MB). Requests exceeding this get 413. */
  maxBodyBytes?: number;
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
  const maxBodyBytes = options?.maxBodyBytes ?? 1_048_576; // 1MB

  return (req: IncomingMessage, res: ServerResponse) => {
    // Parse URL path (handles both origin-form and absolute-form request targets)
    const { pathname } = new URL(req.url ?? "/", "http://localhost");

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
    let totalBytes = 0;
    req.on("data", (chunk: Buffer) => {
      totalBytes += chunk.length;
      if (totalBytes <= maxBodyBytes) {
        chunks.push(chunk);
      }
    });
    req.on("end", () => {
      if (totalBytes > maxBodyBytes) {
        res.writeHead(413);
        res.end("Payload Too Large");
        return;
      }
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
