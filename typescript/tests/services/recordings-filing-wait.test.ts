import { describe, it, expect } from "vitest";
import { http, HttpResponse } from "msw";
import { server } from "../setup.js";
import { createBasecampClient } from "../../src/client.js";
import {
  FilingUnsettledError,
  filingContainerFor,
  filingContainerTakesFolders,
} from "../../src/services/filing-wait.js";

const BASE_URL = "https://3.basecampapi.com/12345";
const filing = (status: string) => ({
  id: 4,
  status,
  type: "Copy",
  source_recording_id: 3001,
  url: `${BASE_URL}/filings/4.json`,
});

describe("RecordingsService.copyAndWait", () => {
  const recordings = () =>
    createBasecampClient({ accountId: "12345", accessToken: "test-token" }).recordings;

  it("ends the wait with the filing still processing", async () => {
    let starts = 0;
    server.use(
      http.post(`${BASE_URL}/recordings/3001/copies.json`, () => {
        starts++;
        return HttpResponse.json(filing("pending"), { status: 201 });
      }),
      http.get(`${BASE_URL}/filings/4`, () => HttpResponse.json(filing("processing"))),
    );

    const err = await recordings()
      .copyAndWait(3001, { filingDestinationId: 789 }, { timeoutMs: 600 })
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(FilingUnsettledError);
    expect((err as FilingUnsettledError).filing).toMatchObject({ id: 4, status: "processing" });
    expect((err as FilingUnsettledError).retryable).toBe(false);
    expect(starts).toBe(1);
  });

  it("reports a failed poll as unsettled, not as a refusal", async () => {
    server.use(
      http.post(`${BASE_URL}/recordings/3001/copies.json`, () =>
        HttpResponse.json(filing("pending"), { status: 201 }),
      ),
      http.get(`${BASE_URL}/filings/4`, () =>
        HttpResponse.json({ status: 404, error: "Not Found" }, { status: 404 }),
      ),
    );

    const err = await recordings()
      .copyAndWait(3001, { filingDestinationId: 789 })
      .catch((e: unknown) => e);
    expect(err).toBeInstanceOf(FilingUnsettledError);
    expect((err as FilingUnsettledError).code).toBe("api_error");
  });
});

describe("filingContainerFor", () => {
  it.each([
    ["Kanban::Card", "Kanban::Triage", "kanban_board"],
    ["Kanban::Column", "Kanban::Board", "kanban_board"],
    ["Message", "Message::Board", "message_board"],
    ["Document", "Vault", "vault"],
    ["Upload", "Vault", "vault"],
    ["CloudFile", "Vault", "vault"],
    ["GoogleDocument", "Vault", "vault"],
    ["Vault", "Vault", "vault"],
    ["Schedule::Entry", "Schedule", "schedule"],
    ["Question", "Questionnaire", "questionnaire"],
    ["Inbox::Forward", "Inbox", "inbox"],
    ["Todolist", "Todoset", "todoset"],
    ["Todolist", "Todolist", "todolist"],
    ["Todo", "Todolist", "todolist"],
    ["Vault", "", undefined],
    ["Message::Board", "", undefined],
    ["Kanban::Triage", "Kanban::Board", undefined],
    ["Comment", "Message", undefined],
    ["Question::Answer", "Question", undefined],
  ])("%s under %s files into %s", (recording, parent, want) => {
    const got = filingContainerFor(recording, parent);
    expect(got).toBe(want);
    if (got) expect(filingContainerTakesFolders(got)).toBe(got === "vault");
  });
});
