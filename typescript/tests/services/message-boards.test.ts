/**
 * Tests for the MessageBoardsService class
 */
import { describe, it, expect, beforeEach } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient, type BasecampClient } from "../../src/client.js";
import { BasecampError } from "../../src/errors.js";

const BASE_URL = "https://3.basecampapi.com/12345";

describe("MessageBoardsService", () => {
  let client: BasecampClient;

  beforeEach(() => {
    client = createBasecampClient({
      accountId: "12345",
      accessToken: "test-token",
    });
  });

  describe("get", () => {
    it("should get a message board by ID", async () => {
      const mockBoard = {
        id: 123,
        status: "active",
        title: "Message Board",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-02T00:00:00Z",
        type: "Message::Board",
        url: "https://3.basecampapi.com/12345/message_boards/123.json",
        app_url: "https://3.basecamp.com/12345/message_boards/123",
        messages_count: 5,
        messages_url: "https://3.basecampapi.com/12345/message_boards/123/messages.json",
        bucket: { id: 1, name: "Test Project", type: "Project" },
        creator: { id: 999, name: "Test User", email_address: "test@example.com" },
      };

      server.use(
        http.get(`${BASE_URL}/message_boards/123`, () => {
          return HttpResponse.json(mockBoard);
        })
      );

      const board = await client.messageBoards.get(123);

      expect(board.id).toBe(123);
      expect(board.title).toBe("Message Board");
      expect(board.messages_count).toBe(5);
      expect(board.bucket?.id).toBe(1);
      expect(board.creator?.name).toBe("Test User");
    });

    it("should throw not_found for non-existent board", async () => {
      server.use(
        http.get(`${BASE_URL}/message_boards/999`, () => {
          return HttpResponse.json({ error: "Not found" }, { status: 404 });
        })
      );

      try {
        await client.messageBoards.get(999);
        expect.fail("Should have thrown");
      } catch (err) {
        expect(err).toBeInstanceOf(BasecampError);
        expect((err as BasecampError).code).toBe("not_found");
      }
    });
  });
});
