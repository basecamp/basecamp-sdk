/**
 * Tests for the EverythingService (generated from OpenAPI spec)
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";
import type { BasecampClient } from "../../src/client.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("EverythingService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
      enableRetry: false,
    });
  });

  describe("everythingFiles", () => {
    it("encodes the kind filter and repeatable people_ids[] in the query string", async () => {
      let captured = "";
      server.use(
        http.get(`${BASE_URL}/files.json`, ({ request }) => {
          captured = new URL(request.url).search;
          return HttpResponse.json([]);
        })
      );

      await client.everything.everythingFiles({ kind: "images", peopleIds: [11, 22] });

      const params = new URLSearchParams(captured);
      expect(params.get("kind")).toBe("images");
      // people_ids[] is a repeatable array param: both ids must appear under the
      // bracketed key, not a single comma-joined value.
      expect(params.getAll("people_ids[]")).toEqual(["11", "22"]);
    });

    it("should decode the heterogeneous /files.json feed: Upload, Document, and Attachment variants in one array", async () => {
      const fixture = [
        {
          id: 900,
          type: "Upload",
          status: "active",
          visible_to_clients: false,
          title: "logo.png",
          inherits_status: true,
          filename: "logo.png",
          content_type: "image/png",
          byte_size: 1281,
          width: 1024.0,
          height: 768.0,
          url: "https://3.basecampapi.com/1/buckets/2/uploads/900.json",
          app_url: "https://3.basecamp.com/1/buckets/2/uploads/900",
          download_url: "https://3.basecampapi.com/1/buckets/2/uploads/900/download/logo.png",
          app_download_url: "https://storage.3.basecamp.com/1/buckets/2/uploads/900/download/logo.png",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          creator: { id: 1, name: "Victor Cooper" },
        },
        {
          id: 901,
          type: "Document",
          status: "active",
          visible_to_clients: false,
          title: "Spec",
          inherits_status: true,
          content_type: "text/html",
          url: "https://3.basecampapi.com/1/buckets/2/documents/901.json",
          app_url: "https://3.basecamp.com/1/buckets/2/documents/901",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          creator: { id: 1, name: "Victor Cooper" },
        },
        {
          id: 902,
          type: "Attachment",
          attachable_sgid: "sgid-902",
          filename: "chart.avif",
          content_type: "image/avif",
          byte_size: 4096,
          width: null,
          height: null,
          download_url: "https://storage.3.basecamp.com/1/blobs/902/download/chart.avif",
          parent: { id: 800, title: "A message", type: "Message" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/files.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingFiles()) as any[];
      expect(result).toHaveLength(3);

      // variant 1: full Upload recording
      expect(result[0].type).toBe("Upload");
      expect(result[0].filename).toBe("logo.png");
      expect(result[0].app_download_url).toBe(
        "https://storage.3.basecamp.com/1/buckets/2/uploads/900/download/logo.png"
      );
      expect(result[0].width).toBe(1024);

      // variant 2: Basecamp Document recording
      expect(result[1].type).toBe("Document");
      expect(result[1].title).toBe("Spec");

      // variant 3: rich-text Attachment envelope
      expect(result[2].type).toBe("Attachment");
      expect(result[2].attachable_sgid).toBe("sgid-902");
      expect(result[2].parent).toBeDefined();
      expect(result[2].width).toBeNull();
    });
  });

  describe("everythingMessages", () => {
    it("should decode the paginated /messages.json Recording feed with embedded buckets", async () => {
      const fixture = [
        {
          id: 1001,
          type: "Message",
          title: "Kickoff",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          creator: { id: 1, name: "Victor Cooper" },
        },
        {
          id: 1002,
          type: "Message",
          title: "Status update",
          bucket: { id: 3, name: "Honcho Rollout", type: "Project" },
          creator: { id: 1, name: "Victor Cooper" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/messages.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingMessages()) as any[];
      expect(result).toHaveLength(2);
      expect(result[0].id).toBe(1001);
      expect(result[0].bucket).toEqual({ id: 2, name: "The Leto Laptop", type: "Project" });
      expect(result[1].id).toBe(1002);
      expect(result[1].bucket.id).toBe(3);
    });
  });

  describe("everythingComments", () => {
    it("should decode the /comments.json Recording feed with embedded buckets", async () => {
      const fixture = [
        {
          id: 2001,
          type: "Comment",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          creator: { id: 1, name: "Victor Cooper" },
        },
        {
          id: 2002,
          type: "Comment",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          creator: { id: 5, name: "Annie Bryan" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/comments.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingComments()) as any[];
      expect(result).toHaveLength(2);
      expect(result[0].id).toBe(2001);
      expect(result[0].bucket).toEqual({ id: 2, name: "The Leto Laptop", type: "Project" });
      expect(result[1].id).toBe(2002);
    });
  });

  describe("everythingCheckins", () => {
    it("should decode the /checkins.json Recording feed with embedded buckets", async () => {
      const fixture = [
        {
          id: 3001,
          type: "Question::Answer",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          creator: { id: 1, name: "Victor Cooper" },
        },
        {
          id: 3002,
          type: "Question::Answer",
          bucket: { id: 4, name: "Marketing Site", type: "Project" },
          creator: { id: 5, name: "Annie Bryan" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/checkins.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingCheckins()) as any[];
      expect(result).toHaveLength(2);
      expect(result[0].id).toBe(3001);
      expect(result[0].bucket).toEqual({ id: 2, name: "The Leto Laptop", type: "Project" });
      expect(result[1].bucket.id).toBe(4);
    });
  });

  describe("everythingForwards", () => {
    it("should decode the /forwards.json Recording feed with embedded buckets", async () => {
      const fixture = [
        {
          id: 4001,
          type: "Inbox::Forward",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          creator: { id: 1, name: "Victor Cooper" },
        },
        {
          id: 4002,
          type: "Inbox::Forward",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          creator: { id: 5, name: "Annie Bryan" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/forwards.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingForwards()) as any[];
      expect(result).toHaveLength(2);
      expect(result[0].id).toBe(4001);
      expect(result[0].bucket).toEqual({ id: 2, name: "The Leto Laptop", type: "Project" });
      expect(result[1].id).toBe(4002);
    });
  });

  describe("everythingOverdueTodos", () => {
    it("should decode the unpaginated /todos/overdue.json array, oldest-first", async () => {
      const fixture = [
        {
          id: 6001,
          type: "Todo",
          title: "Ship the thing",
          due_on: "2024-01-10",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
        },
        {
          id: 6002,
          type: "Todo",
          title: "Review the docs",
          due_on: "2024-01-20",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/todos/overdue.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingOverdueTodos()) as any[];
      expect(result).toHaveLength(2);
      expect(result[0].id).toBe(6001);
      expect(result[1].id).toBe(6002);
      // oldest-first ordering by due_on
      expect(result[0].due_on).toBe("2024-01-10");
      expect(result[1].due_on).toBe("2024-01-20");
      expect(result[0].due_on! < result[1].due_on!).toBe(true);
    });
  });

  describe("everythingOverdueCards", () => {
    it("should decode the unpaginated /cards/overdue.json array, oldest-first", async () => {
      const fixture = [
        {
          id: 7001,
          type: "Kanban::Card",
          title: "Draft proposal",
          due_on: "2024-02-01",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
        },
        {
          id: 7002,
          type: "Kanban::Card",
          title: "Send invoice",
          due_on: "2024-02-14",
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
        },
      ];

      server.use(
        http.get(`${BASE_URL}/cards/overdue.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingOverdueCards()) as any[];
      expect(result).toHaveLength(2);
      expect(result[0].id).toBe(7001);
      expect(result[1].id).toBe(7002);
      // oldest-first ordering by due_on
      expect(result[0].due_on).toBe("2024-02-01");
      expect(result[1].due_on).toBe("2024-02-14");
      expect(result[0].due_on! < result[1].due_on!).toBe(true);
    });
  });

  describe("error propagation", () => {
    const cases: Array<[string, string, () => Promise<unknown>]> = [
      ["everythingMessages", "/messages.json", () => client.everything.everythingMessages()],
      ["everythingComments", "/comments.json", () => client.everything.everythingComments()],
      ["everythingCheckins", "/checkins.json", () => client.everything.everythingCheckins()],
      ["everythingForwards", "/forwards.json", () => client.everything.everythingForwards()],
      ["everythingFiles", "/files.json", () => client.everything.everythingFiles()],
      ["everythingOverdueTodos", "/todos/overdue.json", () => client.everything.everythingOverdueTodos()],
      ["everythingOverdueCards", "/cards/overdue.json", () => client.everything.everythingOverdueCards()],
      ["everythingOpenTodos", "/todos/open.json", () => client.everything.everythingOpenTodos()],
      ["everythingCompletedTodos", "/todos/completed.json", () => client.everything.everythingCompletedTodos()],
      ["everythingUnassignedTodos", "/todos/unassigned.json", () => client.everything.everythingUnassignedTodos()],
      ["everythingNoDueDateTodos", "/todos/no_due_date.json", () => client.everything.everythingNoDueDateTodos()],
      ["everythingOpenCards", "/cards/open.json", () => client.everything.everythingOpenCards()],
      ["everythingCompletedCards", "/cards/completed.json", () => client.everything.everythingCompletedCards()],
      ["everythingUnassignedCards", "/cards/unassigned.json", () => client.everything.everythingUnassignedCards()],
      ["everythingNoDueDateCards", "/cards/no_due_date.json", () => client.everything.everythingNoDueDateCards()],
      ["everythingNotNowCards", "/cards/not_now.json", () => client.everything.everythingNotNowCards()],
    ];

    it.each(cases)("%s propagates a 4xx as a BasecampError", async (_name, path, call) => {
      server.use(
        http.get(`${BASE_URL}${path}`, () => HttpResponse.json({ error: "Not found" }, { status: 404 }))
      );

      await expect(call()).rejects.toThrow(BasecampError);
    });
  });

  describe("everythingOpenTodos", () => {
    it("encodes the due filter and repeatable assignee_ids[] in the query string", async () => {
      let captured = "";
      server.use(
        http.get(`${BASE_URL}/todos/open.json`, ({ request }) => {
          captured = new URL(request.url).search;
          return HttpResponse.json([]);
        })
      );

      await client.everything.everythingOpenTodos({ assigneeIds: [11, 22], due: "overdue" });

      const params = new URLSearchParams(captured);
      expect(params.get("due")).toBe("overdue");
      // assignee_ids[] is a repeatable array param: both ids must appear under
      // the bracketed key, not a single comma-joined value.
      expect(params.getAll("assignee_ids[]")).toEqual(["11", "22"]);
    });

    it("encodes the filters on the unpaginated overdue feed too", async () => {
      let captured = "";
      server.use(
        http.get(`${BASE_URL}/todos/overdue.json`, ({ request }) => {
          captured = new URL(request.url).search;
          return HttpResponse.json([]);
        })
      );

      await client.everything.everythingOverdueTodos({ assigneeIds: [7], due: "with" });

      const params = new URLSearchParams(captured);
      expect(params.getAll("assignee_ids[]")).toEqual(["7"]);
      expect(params.get("due")).toBe("with");
    });

    it("should decode the /todos/open.json feed grouped by bucket, each to-do carrying its steps", async () => {
      const fixture = [
        {
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          todos: [
            {
              id: 8001,
              type: "Todo",
              title: "Draft the spec",
              bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
              steps: [{ id: 90011, type: "Todo", title: "Outline sections" }],
            },
          ],
        },
        {
          bucket: { id: 3, name: "Honcho Rollout", type: "Project" },
          todos: [
            {
              id: 8002,
              type: "Todo",
              title: "Book the venue",
              bucket: { id: 3, name: "Honcho Rollout", type: "Project" },
              steps: [{ id: 90021, type: "Todo", title: "Compare quotes" }],
            },
          ],
        },
      ];

      server.use(
        http.get(`${BASE_URL}/todos/open.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingOpenTodos()) as any[];
      expect(result).toHaveLength(2);
      expect(result[0].bucket).toEqual({ id: 2, name: "The Leto Laptop", type: "Project" });
      expect(result[0].todos).toHaveLength(1);
      expect(result[0].todos[0].id).toBe(8001);
      expect(result[0].todos[0].steps[0].title).toBe("Outline sections");
      expect(result[1].bucket.id).toBe(3);
      expect(result[1].todos[0].id).toBe(8002);
    });
  });

  describe("everythingCompletedTodos", () => {
    it("should decode the /todos/completed.json feed grouped by bucket, each to-do carrying its steps", async () => {
      const fixture = [
        {
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          todos: [
            {
              id: 8101,
              type: "Todo",
              title: "Ship v1",
              completed: true,
              bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
              steps: [{ id: 91011, type: "Todo", title: "Tag release", completed: true }],
            },
          ],
        },
        {
          bucket: { id: 4, name: "Marketing Site", type: "Project" },
          todos: [
            {
              id: 8102,
              type: "Todo",
              title: "Publish post",
              completed: true,
              bucket: { id: 4, name: "Marketing Site", type: "Project" },
              steps: [{ id: 91021, type: "Todo", title: "Proofread", completed: true }],
            },
          ],
        },
      ];

      server.use(
        http.get(`${BASE_URL}/todos/completed.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingCompletedTodos()) as any[];
      expect(result).toHaveLength(2);
      expect(result[0].bucket).toEqual({ id: 2, name: "The Leto Laptop", type: "Project" });
      expect(result[0].todos).toHaveLength(1);
      expect(result[0].todos[0].id).toBe(8101);
      expect(result[0].todos[0].steps[0].title).toBe("Tag release");
      expect(result[1].bucket.id).toBe(4);
      expect(result[1].todos[0].id).toBe(8102);
    });
  });

  describe("everythingUnassignedTodos", () => {
    it("should decode the /todos/unassigned.json feed grouped by bucket, each to-do carrying its steps", async () => {
      const fixture = [
        {
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          todos: [
            {
              id: 8201,
              type: "Todo",
              title: "Assign an owner",
              bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
              assignees: [],
              steps: [{ id: 92011, type: "Todo", title: "Pick a lead" }],
            },
          ],
        },
      ];

      server.use(
        http.get(`${BASE_URL}/todos/unassigned.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingUnassignedTodos()) as any[];
      expect(result).toHaveLength(1);
      expect(result[0].bucket).toEqual({ id: 2, name: "The Leto Laptop", type: "Project" });
      expect(result[0].todos).toHaveLength(1);
      expect(result[0].todos[0].id).toBe(8201);
      expect(result[0].todos[0].assignees).toEqual([]);
      expect(result[0].todos[0].steps[0].title).toBe("Pick a lead");
    });
  });

  describe("everythingNoDueDateTodos", () => {
    it("should decode the /todos/no_due_date.json feed grouped by bucket, each to-do carrying its steps", async () => {
      const fixture = [
        {
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          todos: [
            {
              id: 8301,
              type: "Todo",
              title: "Someday task",
              due_on: null,
              bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
              steps: [{ id: 93011, type: "Todo", title: "Scope it" }],
            },
          ],
        },
      ];

      server.use(
        http.get(`${BASE_URL}/todos/no_due_date.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingNoDueDateTodos()) as any[];
      expect(result).toHaveLength(1);
      expect(result[0].bucket).toEqual({ id: 2, name: "The Leto Laptop", type: "Project" });
      expect(result[0].todos).toHaveLength(1);
      expect(result[0].todos[0].id).toBe(8301);
      expect(result[0].todos[0].due_on).toBeNull();
      expect(result[0].todos[0].steps[0].title).toBe("Scope it");
    });
  });

  describe("everythingOpenCards", () => {
    it("should decode the /cards/open.json feed grouped by bucket, each card carrying its steps", async () => {
      const fixture = [
        {
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          cards: [
            {
              id: 8401,
              type: "Kanban::Card",
              title: "Design review",
              bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
              steps: [{ id: 94011, type: "Kanban::Step", title: "Gather feedback" }],
            },
          ],
        },
        {
          bucket: { id: 3, name: "Honcho Rollout", type: "Project" },
          cards: [
            {
              id: 8402,
              type: "Kanban::Card",
              title: "Wire the API",
              bucket: { id: 3, name: "Honcho Rollout", type: "Project" },
              steps: [{ id: 94021, type: "Kanban::Step", title: "Define routes" }],
            },
          ],
        },
      ];

      server.use(
        http.get(`${BASE_URL}/cards/open.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingOpenCards()) as any[];
      expect(result).toHaveLength(2);
      expect(result[0].bucket).toEqual({ id: 2, name: "The Leto Laptop", type: "Project" });
      expect(result[0].cards).toHaveLength(1);
      expect(result[0].cards[0].id).toBe(8401);
      expect(result[0].cards[0].steps[0].title).toBe("Gather feedback");
      expect(result[1].bucket.id).toBe(3);
      expect(result[1].cards[0].id).toBe(8402);
    });
  });

  describe("everythingCompletedCards", () => {
    it("should decode the /cards/completed.json feed grouped by bucket, each card carrying its steps", async () => {
      const fixture = [
        {
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          cards: [
            {
              id: 8501,
              type: "Kanban::Card",
              title: "Launch checklist",
              completed: true,
              bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
              steps: [{ id: 95011, type: "Kanban::Step", title: "Verify DNS", completed: true }],
            },
          ],
        },
      ];

      server.use(
        http.get(`${BASE_URL}/cards/completed.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingCompletedCards()) as any[];
      expect(result).toHaveLength(1);
      expect(result[0].bucket).toEqual({ id: 2, name: "The Leto Laptop", type: "Project" });
      expect(result[0].cards).toHaveLength(1);
      expect(result[0].cards[0].id).toBe(8501);
      expect(result[0].cards[0].steps[0].title).toBe("Verify DNS");
    });
  });

  describe("everythingUnassignedCards", () => {
    it("should decode the /cards/unassigned.json feed grouped by bucket, each card carrying its steps", async () => {
      const fixture = [
        {
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          cards: [
            {
              id: 8601,
              type: "Kanban::Card",
              title: "Needs an owner",
              bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
              assignees: [],
              steps: [{ id: 96011, type: "Kanban::Step", title: "Triage" }],
            },
          ],
        },
      ];

      server.use(
        http.get(`${BASE_URL}/cards/unassigned.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingUnassignedCards()) as any[];
      expect(result).toHaveLength(1);
      expect(result[0].bucket).toEqual({ id: 2, name: "The Leto Laptop", type: "Project" });
      expect(result[0].cards).toHaveLength(1);
      expect(result[0].cards[0].id).toBe(8601);
      expect(result[0].cards[0].assignees).toEqual([]);
      expect(result[0].cards[0].steps[0].title).toBe("Triage");
    });
  });

  describe("everythingNoDueDateCards", () => {
    it("should decode the /cards/no_due_date.json feed grouped by bucket, each card carrying its steps", async () => {
      const fixture = [
        {
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          cards: [
            {
              id: 8701,
              type: "Kanban::Card",
              title: "Backlog idea",
              due_on: null,
              bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
              steps: [{ id: 97011, type: "Kanban::Step", title: "Sketch it" }],
            },
          ],
        },
      ];

      server.use(
        http.get(`${BASE_URL}/cards/no_due_date.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingNoDueDateCards()) as any[];
      expect(result).toHaveLength(1);
      expect(result[0].bucket).toEqual({ id: 2, name: "The Leto Laptop", type: "Project" });
      expect(result[0].cards).toHaveLength(1);
      expect(result[0].cards[0].id).toBe(8701);
      expect(result[0].cards[0].due_on).toBeNull();
      expect(result[0].cards[0].steps[0].title).toBe("Sketch it");
    });
  });

  describe("everythingNotNowCards", () => {
    it("should decode the /cards/not_now.json feed grouped by bucket, each card carrying its steps", async () => {
      const fixture = [
        {
          bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
          cards: [
            {
              id: 8801,
              type: "Kanban::Card",
              title: "Parked for later",
              bucket: { id: 2, name: "The Leto Laptop", type: "Project" },
              steps: [{ id: 98011, type: "Kanban::Step", title: "Revisit next quarter" }],
            },
          ],
        },
        {
          bucket: { id: 4, name: "Marketing Site", type: "Project" },
          cards: [
            {
              id: 8802,
              type: "Kanban::Card",
              title: "On hold",
              bucket: { id: 4, name: "Marketing Site", type: "Project" },
              steps: [{ id: 98021, type: "Kanban::Step", title: "Await budget" }],
            },
          ],
        },
      ];

      server.use(
        http.get(`${BASE_URL}/cards/not_now.json`, () => {
          return HttpResponse.json(fixture);
        })
      );

      const result = (await client.everything.everythingNotNowCards()) as any[];
      expect(result).toHaveLength(2);
      expect(result[0].bucket).toEqual({ id: 2, name: "The Leto Laptop", type: "Project" });
      expect(result[0].cards).toHaveLength(1);
      expect(result[0].cards[0].id).toBe(8801);
      expect(result[0].cards[0].steps[0].title).toBe("Revisit next quarter");
      expect(result[1].bucket.id).toBe(4);
      expect(result[1].cards[0].id).toBe(8802);
    });
  });
});
