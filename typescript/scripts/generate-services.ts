#!/usr/bin/env node
/**
 * Generates TypeScript service classes from OpenAPI spec.
 *
 * Usage: npx tsx scripts/generate-services.ts [--openapi ../openapi.json] [--output src/generated/services]
 *
 * This generator:
 * 1. Parses openapi.json
 * 2. Groups operations by tag
 * 3. Maps operationIds to method names
 * 4. Generates TypeScript service files
 */

import * as fs from "fs";
import * as path from "path";

// =============================================================================
// Types
// =============================================================================

interface OpenAPISpec {
  openapi: string;
  info: { title: string; version: string };
  paths: Record<string, PathItem>;
  components: {
    schemas: Record<string, Schema>;
  };
}

interface PathItem {
  [method: string]: Operation | undefined;
}

interface Operation {
  operationId: string;
  description?: string;
  tags?: string[];
  parameters?: Parameter[];
  requestBody?: RequestBody;
  responses?: Record<string, Response>;
  "x-basecamp-pagination"?: {
    style: string;
    maxPageSize?: number;
    totalCountHeader?: string;
  };
}

interface Parameter {
  name: string;
  in: "path" | "query" | "header";
  description?: string;
  required?: boolean;
  schema: Schema;
}

interface RequestBody {
  content?: {
    "application/json"?: { schema: Schema };
    "application/octet-stream"?: { schema: Schema };
  };
  required?: boolean;
}

interface Response {
  description: string;
  content?: {
    "application/json"?: { schema: Schema };
  };
}

interface Schema {
  type?: string;
  format?: string;
  $ref?: string;
  properties?: Record<string, Schema>;
  required?: string[];
  items?: Schema;
}

interface ParsedOperation {
  operationId: string;
  methodName: string;
  httpMethod: "GET" | "POST" | "PUT" | "DELETE" | "PATCH";
  path: string;
  description: string;
  pathParams: PathParam[];
  queryParams: QueryParam[];
  bodySchema?: Schema;
  bodyRequired: boolean;
  bodyContentType?: "json" | "octet-stream";
  responseSchema?: Schema;
  returnsArray: boolean;
  returnsVoid: boolean;
  isMutation: boolean;
  resourceType: string;
  wrapperKey?: string;
}

interface PathParam {
  name: string;
  type: string;
}

interface QueryParam {
  name: string;
  type: string;
  required: boolean;
  description?: string;
}

interface ServiceDefinition {
  name: string;
  className: string;
  description: string;
  operations: ParsedOperation[];
  types: Set<string>;
}

// =============================================================================
// Configuration
// =============================================================================

/**
 * Tag to service name mapping overrides.
 * By default, tag name is used directly.
 */
const TAG_TO_SERVICE: Record<string, string> = {
  "Card Tables": "CardTables",
  Campfire: "Campfires",
  Todos: "Todos",
  Messages: "Messages",
  Files: "Files",
  Forwards: "Forwards",
  Schedule: "Schedules",
  People: "People",
  Projects: "Projects",
  Automation: "Automation",
  ClientFeatures: "ClientFeatures",
  Untagged: "Miscellaneous",
};

/**
 * Service split configuration.
 * Some tags map to multiple service classes.
 */
const SERVICE_SPLITS: Record<string, Record<string, string[]>> = {
  Campfire: {
    Campfires: [
      "GetCampfire",
      "ListCampfires",
      "ListChatbots",
      "CreateChatbot",
      "GetChatbot",
      "UpdateChatbot",
      "DeleteChatbot",
      "ListCampfireLines",
      "CreateCampfireLine",
      "GetCampfireLine",
      "DeleteCampfireLine",
    ],
  },
  "Card Tables": {
    CardTables: ["GetCardTable"],
    Cards: [
      "GetCard",
      "UpdateCard",
      "MoveCard",
      "CreateCard",
      "ListCards",
    ],
    CardColumns: [
      "GetCardColumn",
      "UpdateCardColumn",
      "SetCardColumnColor",
      "EnableCardColumnOnHold",
      "DisableCardColumnOnHold",
      "CreateCardColumn",
      "MoveCardColumn",
      "SubscribeToCardColumn",
      "UnsubscribeFromCardColumn",
    ],
    CardSteps: [
      "CreateCardStep",
      "UpdateCardStep",
      "CompleteCardStep",
      "UncompleteCardStep",
      "RepositionCardStep",
    ],
  },
  Files: {
    Attachments: ["CreateAttachment"],
    Uploads: [
      "GetUpload",
      "UpdateUpload",
      "ListUploads",
      "CreateUpload",
      "ListUploadVersions",
    ],
    Vaults: [
      "GetVault",
      "UpdateVault",
      "ListVaults",
      "CreateVault",
    ],
    Documents: [
      "GetDocument",
      "UpdateDocument",
      "ListDocuments",
      "CreateDocument",
    ],
  },
  Automation: {
    Tools: [
      "GetTool",
      "UpdateTool",
      "DeleteTool",
      "CloneTool",
      "EnableTool",
      "DisableTool",
      "RepositionTool",
    ],
    Recordings: [
      "GetRecording",
      "ArchiveRecording",
      "UnarchiveRecording",
      "TrashRecording",
      "ListRecordings",
    ],
    Webhooks: [
      "ListWebhooks",
      "CreateWebhook",
      "GetWebhook",
      "UpdateWebhook",
      "DeleteWebhook",
    ],
    Events: ["ListEvents"],
    Lineup: [
      "CreateLineupMarker",
      "UpdateLineupMarker",
      "DeleteLineupMarker",
    ],
    Search: ["Search", "GetSearchMetadata"],
    Templates: [
      "ListTemplates",
      "CreateTemplate",
      "GetTemplate",
      "UpdateTemplate",
      "DeleteTemplate",
      "CreateProjectFromTemplate",
      "GetProjectConstruction",
    ],
    Checkins: [
      "GetQuestionnaire",
      "ListQuestions",
      "CreateQuestion",
      "GetQuestion",
      "UpdateQuestion",
      "ListAnswers",
      "CreateAnswer",
      "GetAnswer",
      "UpdateAnswer",
    ],
  },
  Messages: {
    Messages: [
      "GetMessage",
      "UpdateMessage",
      "CreateMessage",
      "ListMessages",
      "PinMessage",
      "UnpinMessage",
    ],
    MessageBoards: ["GetMessageBoard"],
    MessageTypes: [
      "ListMessageTypes",
      "CreateMessageType",
      "GetMessageType",
      "UpdateMessageType",
      "DeleteMessageType",
    ],
    Comments: [
      "GetComment",
      "UpdateComment",
      "ListComments",
      "CreateComment",
    ],
  },
  People: {
    People: [
      "GetMyProfile",
      "ListPeople",
      "GetPerson",
      "ListProjectPeople",
      "UpdateProjectAccess",
      "ListPingablePeople",
      "ListAssignablePeople",
    ],
    Subscriptions: [
      "GetSubscription",
      "Subscribe",
      "Unsubscribe",
      "UpdateSubscription",
    ],
  },
  Schedule: {
    Schedules: [
      "GetSchedule",
      "UpdateScheduleSettings",
      "ListScheduleEntries",
      "CreateScheduleEntry",
      "GetScheduleEntry",
      "UpdateScheduleEntry",
      "GetScheduleEntryOccurrence",
    ],
    Timesheets: [
      "GetRecordingTimesheet",
      "GetProjectTimesheet",
      "GetTimesheetReport",
    ],
  },
  ClientFeatures: {
    ClientApprovals: ["ListClientApprovals", "GetClientApproval"],
    ClientCorrespondences: ["ListClientCorrespondences", "GetClientCorrespondence"],
    ClientReplies: ["ListClientReplies", "GetClientReply"],
    ClientVisibility: ["SetClientVisibility"],
  },
  Todos: {
    Todos: [
      "ListTodos",
      "CreateTodo",
      "GetTodo",
      "UpdateTodo",
      "CompleteTodo",
      "UncompleteTodo",
      "TrashTodo",
    ],
    Todolists: [
      "GetTodolistOrGroup",
      "UpdateTodolistOrGroup",
      "ListTodolists",
      "CreateTodolist",
    ],
    Todosets: ["GetTodoset"],
    TodolistGroups: [
      "ListTodolistGroups",
      "CreateTodolistGroup",
      "RepositionTodolistGroup",
    ],
  },
  Untagged: {
    Timeline: ["GetProjectTimeline"],
    Reports: [
      "GetProgressReport",
      "GetUpcomingSchedule",
      "GetAssignedTodos",
      "GetOverdueTodos",
      "GetPersonProgress",
    ],
    Checkins: [
      "GetQuestionReminders",
      "ListQuestionAnswerers",
      "GetAnswersByPerson",
      "UpdateQuestionNotificationSettings",
      "PauseQuestion",
      "ResumeQuestion",
    ],
    Todos: [
      "RepositionTodo",
    ],
    People: [
      "ListAssignablePeople",
    ],
    CardColumns: [
      "SubscribeToCardColumn",
      "UnsubscribeFromCardColumn",
    ],
  },
};

/**
 * Verb extraction patterns for operationId → method name mapping.
 */
const VERB_PATTERNS = [
  { prefix: "Subscribe", method: "subscribe" },
  { prefix: "Unsubscribe", method: "unsubscribe" },
  { prefix: "List", method: "list" },
  { prefix: "Get", method: "get" },
  { prefix: "Create", method: "create" },
  { prefix: "Update", method: "update" },
  { prefix: "Delete", method: "delete" },
  { prefix: "Trash", method: "trash" },
  { prefix: "Archive", method: "archive" },
  { prefix: "Unarchive", method: "unarchive" },
  { prefix: "Complete", method: "complete" },
  { prefix: "Uncomplete", method: "uncomplete" },
  { prefix: "Enable", method: "enable" },
  { prefix: "Disable", method: "disable" },
  { prefix: "Reposition", method: "reposition" },
  { prefix: "Move", method: "move" },
  { prefix: "Clone", method: "clone" },
  { prefix: "Set", method: "set" },
  { prefix: "Pin", method: "pin" },
  { prefix: "Unpin", method: "unpin" },
  { prefix: "Pause", method: "pause" },
  { prefix: "Resume", method: "resume" },
  { prefix: "Search", method: "search" },
];

/**
 * Method name overrides for specific operationIds.
 */
const METHOD_NAME_OVERRIDES: Record<string, string> = {
  GetMyProfile: "myProfile",
  GetTodolistOrGroup: "get",
  UpdateTodolistOrGroup: "update",
  SetCardColumnColor: "setColor",
  EnableCardColumnOnHold: "enableOnHold",
  DisableCardColumnOnHold: "disableOnHold",
  RepositionCardStep: "reposition",
  CreateCardStep: "create",
  UpdateCardStep: "update",
  CompleteCardStep: "complete",
  UncompleteCardStep: "uncomplete",
  // Checkins - use specific names to avoid conflicts
  GetQuestionnaire: "getQuestionnaire",
  GetQuestion: "getQuestion",
  GetAnswer: "getAnswer",
  ListQuestions: "listQuestions",
  ListAnswers: "listAnswers",
  CreateQuestion: "createQuestion",
  CreateAnswer: "createAnswer",
  UpdateQuestion: "updateQuestion",
  UpdateAnswer: "updateAnswer",
  GetQuestionReminders: "reminders",
  GetAnswersByPerson: "byPerson",
  ListQuestionAnswerers: "answerers",
  UpdateQuestionNotificationSettings: "updateNotificationSettings",
  PauseQuestion: "pause",
  ResumeQuestion: "resume",
  // Search
  GetSearchMetadata: "metadata",
  Search: "search",
  // Templates
  CreateProjectFromTemplate: "createProject",
  GetProjectConstruction: "getConstruction",
  // Timesheets
  GetRecordingTimesheet: "forRecording",
  GetProjectTimesheet: "forProject",
  GetTimesheetReport: "report",
  // Reports
  GetProgressReport: "progress",
  GetUpcomingSchedule: "upcoming",
  GetAssignedTodos: "assigned",
  GetOverdueTodos: "overdue",
  GetPersonProgress: "personProgress",
  // Card columns
  SubscribeToCardColumn: "subscribeToColumn",
  UnsubscribeFromCardColumn: "unsubscribeFromColumn",
  // Client features
  SetClientVisibility: "setVisibility",
  // Campfires - use specific names to avoid conflicts between campfire, chatbots, and lines
  GetCampfire: "get",
  ListCampfires: "list",
  ListChatbots: "listChatbots",
  CreateChatbot: "createChatbot",
  GetChatbot: "getChatbot",
  UpdateChatbot: "updateChatbot",
  DeleteChatbot: "deleteChatbot",
  ListCampfireLines: "listLines",
  CreateCampfireLine: "createLine",
  GetCampfireLine: "getLine",
  DeleteCampfireLine: "deleteLine",
  // Forwards - use specific names to avoid conflicts between forwards, replies, and inbox
  GetForward: "get",
  ListForwards: "list",
  GetForwardReply: "getReply",
  ListForwardReplies: "listReplies",
  CreateForwardReply: "createReply",
  GetInbox: "getInbox",
  // Uploads - use specific names to avoid conflicts with versions
  GetUpload: "get",
  UpdateUpload: "update",
  ListUploads: "list",
  CreateUpload: "create",
  ListUploadVersions: "listVersions",
  // Messages
  GetMessage: "get",
  UpdateMessage: "update",
  CreateMessage: "create",
  ListMessages: "list",
  PinMessage: "pin",
  UnpinMessage: "unpin",
  // Message board
  GetMessageBoard: "get",
  // Message types
  GetMessageType: "get",
  UpdateMessageType: "update",
  CreateMessageType: "create",
  ListMessageTypes: "list",
  DeleteMessageType: "delete",
  // Comments
  GetComment: "get",
  UpdateComment: "update",
  CreateComment: "create",
  ListComments: "list",
  // People
  ListProjectPeople: "listForProject",
  ListPingablePeople: "listPingable",
  ListAssignablePeople: "listAssignable",
  // Schedules
  GetSchedule: "get",
  UpdateScheduleSettings: "updateSettings",
  GetScheduleEntry: "getEntry",
  UpdateScheduleEntry: "updateEntry",
  CreateScheduleEntry: "createEntry",
  ListScheduleEntries: "listEntries",
  GetScheduleEntryOccurrence: "getEntryOccurrence",
};

/**
 * Response wrapper keys to unwrap.
 */
const RESPONSE_WRAPPER_KEYS: Record<string, string> = {
  todo: "todo",
  todos: "todos",
  todolist: "todolist",
  todolists: "todolists",
  todoset: "todoset",
  message: "message",
  messages: "messages",
  comment: "comment",
  comments: "comments",
  card: "card",
  cards: "cards",
  card_table: "card_table",
  column: "column",
  step: "step",
  project: "project",
  projects: "projects",
  person: "person",
  people: "people",
  campfire: "campfire",
  campfires: "campfires",
  line: "line",
  lines: "lines",
  chatbot: "chatbot",
  chatbots: "chatbots",
  webhook: "webhook",
  webhooks: "webhooks",
  vault: "vault",
  vaults: "vaults",
  document: "document",
  documents: "documents",
  upload: "upload",
  uploads: "uploads",
  schedule: "schedule",
  schedules: "schedules",
  entry: "entry",
  entries: "entries",
  event: "event",
  events: "events",
  recording: "recording",
  recordings: "recordings",
  template: "template",
  templates: "templates",
  attachment: "attachment",
  question: "question",
  questions: "questions",
  answer: "answer",
  answers: "answers",
  questionnaire: "questionnaire",
  subscription: "subscription",
  forward: "forward",
  forwards: "forwards",
  inbox: "inbox",
  message_board: "message_board",
  message_type: "message_type",
  message_types: "message_types",
  tool: "tool",
  marker: "marker",
  correspondence: "correspondence",
  correspondences: "correspondences",
  approval: "approval",
  approvals: "approvals",
  reply: "reply",
  replies: "replies",
  group: "group",
  groups: "groups",
  todolist_group: "todolist_group",
};

// =============================================================================
// Parsing Functions
// =============================================================================

/**
 * Extracts method name from operationId.
 */
function extractMethodName(operationId: string): string {
  // Check for override first
  if (METHOD_NAME_OVERRIDES[operationId]) {
    return METHOD_NAME_OVERRIDES[operationId];
  }

  // Find matching verb pattern
  for (const { prefix, method } of VERB_PATTERNS) {
    if (operationId.startsWith(prefix)) {
      const remainder = operationId.slice(prefix.length);
      if (!remainder) {
        return method;
      }
      // Handle compound names like "GetOverdueTodos" -> "overdueTodos"
      // But simple ones like "GetTodo" -> "get"
      const resource = remainder.charAt(0).toLowerCase() + remainder.slice(1);
      // If the resource is just the entity type, return the verb
      // Otherwise return the combined name
      if (isSimpleResource(resource, operationId)) {
        return method;
      }
      return method === "get" ? resource : method + remainder;
    }
  }

  // Fallback: just lowercase first letter
  return operationId.charAt(0).toLowerCase() + operationId.slice(1);
}

/**
 * Check if resource is a simple entity (should just use the verb).
 */
function isSimpleResource(resource: string, operationId: string): boolean {
  const simpleResources = [
    "todo",
    "todos",
    "todolist",
    "todolists",
    "todoset",
    "message",
    "messages",
    "comment",
    "comments",
    "card",
    "cards",
    "cardtable",
    "cardcolumn",
    "cardstep",
    "column",
    "step",
    "project",
    "projects",
    "person",
    "people",
    "campfire",
    "campfires",
    "chatbot",
    "chatbots",
    "webhook",
    "webhooks",
    "vault",
    "vaults",
    "document",
    "documents",
    "upload",
    "uploads",
    "schedule",
    "scheduleentry",
    "scheduleentries",
    "event",
    "events",
    "recording",
    "recordings",
    "template",
    "templates",
    "attachment",
    "question",
    "questions",
    "answer",
    "answers",
    "questionnaire",
    "subscription",
    "forward",
    "forwards",
    "inbox",
    "messageboard",
    "messagetype",
    "messagetypes",
    "tool",
    "lineupmarker",
    "clientapproval",
    "clientapprovals",
    "clientcorrespondence",
    "clientcorrespondences",
    "clientreply",
    "clientreplies",
    "forwardreply",
    "forwardreplies",
    "campfireline",
    "campfirelines",
    "todolistgroup",
    "todolistgroups",
    "todolistorgroup",
    "uploadversions",
  ];
  return simpleResources.includes(resource.toLowerCase());
}

/**
 * Extracts resource type from operationId.
 */
function extractResourceType(operationId: string): string {
  for (const { prefix } of VERB_PATTERNS) {
    if (operationId.startsWith(prefix)) {
      const remainder = operationId.slice(prefix.length);
      if (!remainder) return "resource";
      // Convert to snake_case
      const snakeCase = remainder
        .replace(/([A-Z])/g, "_$1")
        .toLowerCase()
        .replace(/^_/, "");
      // Singularize: remove trailing 's' but not for words ending in 'ss' (progress, address, etc.)
      return snakeCase.replace(/([^s])s$/, "$1");
    }
  }
  return "resource";
}

/**
 * Resolves a $ref to the schema name.
 */
function resolveRef(ref: string): string {
  // #/components/schemas/Todo -> Todo
  return ref.split("/").pop() || "";
}

/**
 * Converts OpenAPI path to TypeScript path template.
 * Removes {accountId} prefix since it's included in the baseUrl.
 * This matches the hand-written service patterns.
 */
function convertPath(path: string): string {
  // Remove /{accountId} prefix since baseUrl includes it
  return path.replace(/^\/{accountId}/, "");
}

/**
 * Determines if a response is void (no content).
 */
function isVoidResponse(responses: Record<string, Response> | undefined): boolean {
  if (!responses) return true;
  const successResponse = responses["200"] || responses["201"] || responses["204"];
  if (!successResponse) return true;
  return !successResponse.content?.["application/json"];
}

/**
 * Determines if a response is an array.
 */
function isArrayResponse(schema: Schema | undefined): boolean {
  if (!schema) return false;
  if (schema.type === "array") return true;
  if (schema.$ref) {
    // Will need to look up the schema
    const refName = resolveRef(schema.$ref);
    return refName.endsWith("ResponseContent") && refName.includes("List");
  }
  return false;
}

/**
 * Extracts wrapper key from response schema.
 */
function extractWrapperKey(
  schemaName: string,
  schemas: Record<string, Schema>
): string | undefined {
  const schema = schemas[schemaName];
  if (!schema?.properties) return undefined;

  const keys = Object.keys(schema.properties);
  if (keys.length === 1) {
    const key = keys[0];
    if (RESPONSE_WRAPPER_KEYS[key]) {
      return key;
    }
  }
  return undefined;
}

/**
 * Parses a single operation.
 */
function parseOperation(
  path: string,
  method: string,
  operation: Operation,
  schemas: Record<string, Schema>
): ParsedOperation {
  const httpMethod = method.toUpperCase() as "GET" | "POST" | "PUT" | "DELETE" | "PATCH";
  const operationId = operation.operationId;
  const methodName = extractMethodName(operationId);
  const description = operation.description || `${methodName} operation`;

  // Extract path parameters (excluding accountId)
  const pathParams: PathParam[] = (operation.parameters || [])
    .filter((p) => p.in === "path" && p.name !== "accountId")
    .map((p) => ({
      name: p.name,
      type: p.schema.type === "integer" ? "number" : "string",
    }));

  // Extract query parameters
  const queryParams: QueryParam[] = (operation.parameters || [])
    .filter((p) => p.in === "query")
    .map((p) => ({
      name: p.name,
      type: schemaToTsType(p.schema),
      required: p.required || false,
      description: p.description,
    }));

  // Extract request body schema
  let bodySchema: Schema | undefined;
  let bodyRequired = false;
  let bodyContentType: "json" | "octet-stream" | undefined;
  if (operation.requestBody?.content?.["application/json"]?.schema) {
    bodySchema = operation.requestBody.content["application/json"].schema;
    bodyRequired = operation.requestBody.required || false;
    bodyContentType = "json";
  } else if (operation.requestBody?.content?.["application/octet-stream"]?.schema) {
    bodySchema = operation.requestBody.content["application/octet-stream"].schema;
    bodyRequired = operation.requestBody.required || false;
    bodyContentType = "octet-stream";
  }

  // Extract response schema
  let responseSchema: Schema | undefined;
  let wrapperKey: string | undefined;
  const successResponse =
    operation.responses?.["200"] || operation.responses?.["201"];
  if (successResponse?.content?.["application/json"]?.schema) {
    responseSchema = successResponse.content["application/json"].schema;
    if (responseSchema.$ref) {
      const refName = resolveRef(responseSchema.$ref);
      wrapperKey = extractWrapperKey(refName, schemas);
    }
  }

  const returnsVoid = isVoidResponse(operation.responses);
  const returnsArray =
    !returnsVoid && (responseSchema?.type === "array" || isArrayResponse(responseSchema));
  const isMutation = httpMethod !== "GET";
  const resourceType = extractResourceType(operationId);

  return {
    operationId,
    methodName,
    httpMethod,
    path: convertPath(path),
    description,
    pathParams,
    queryParams,
    bodySchema,
    bodyRequired,
    bodyContentType,
    responseSchema,
    returnsArray,
    returnsVoid,
    isMutation,
    resourceType,
    wrapperKey,
  };
}

/**
 * Converts schema to TypeScript type.
 */
function schemaToTsType(schema: Schema): string {
  if (schema.$ref) {
    return resolveRef(schema.$ref);
  }
  switch (schema.type) {
    case "integer":
      return "number";
    case "boolean":
      return "boolean";
    case "array":
      return schema.items ? `${schemaToTsType(schema.items)}[]` : "unknown[]";
    case "object":
      return "Record<string, unknown>";
    default:
      return "string";
  }
}

/**
 * Groups operations into services.
 */
function groupOperations(
  spec: OpenAPISpec
): Map<string, ServiceDefinition> {
  const services = new Map<string, ServiceDefinition>();

  // Parse all operations first
  const operations: Array<{
    op: ParsedOperation;
    tag: string;
    operationId: string;
  }> = [];

  for (const [path, pathItem] of Object.entries(spec.paths)) {
    for (const method of ["get", "post", "put", "patch", "delete"]) {
      const operation = pathItem[method];
      if (!operation) continue;

      const tag = operation.tags?.[0] || "Untagged";
      const parsed = parseOperation(path, method, operation, spec.components.schemas);
      operations.push({ op: parsed, tag, operationId: operation.operationId });
    }
  }

  // Group into services
  for (const { op, tag, operationId } of operations) {
    // Determine which service this operation belongs to
    let serviceName: string;

    // Check if this tag has splits
    if (SERVICE_SPLITS[tag]) {
      // Find the service for this operationId
      let found = false;
      for (const [svc, opIds] of Object.entries(SERVICE_SPLITS[tag])) {
        if (opIds.includes(operationId)) {
          serviceName = svc;
          found = true;
          break;
        }
      }
      if (!found) {
        // Default to tag name
        serviceName = TAG_TO_SERVICE[tag] || tag.replace(/\s+/g, "");
      }
    } else {
      serviceName = TAG_TO_SERVICE[tag] || tag.replace(/\s+/g, "");
    }

    if (!services.has(serviceName)) {
      services.set(serviceName, {
        name: serviceName,
        className: `${serviceName}Service`,
        description: `Service for ${serviceName} operations`,
        operations: [],
        types: new Set(),
      });
    }

    services.get(serviceName)!.operations.push(op);
  }

  return services;
}

// =============================================================================
// Code Generation
// =============================================================================

/**
 * Generates TypeScript code for a service.
 */
function generateService(service: ServiceDefinition): string {
  const lines: string[] = [];

  // File header
  lines.push(`/**`);
  lines.push(` * ${service.description}`);
  lines.push(` *`);
  lines.push(` * @generated from OpenAPI spec`);
  lines.push(` */`);
  lines.push(``);
  lines.push(`import { BaseService } from "../../services/base.js";`);
  lines.push(`import type { components } from "../schema.js";`);
  lines.push(``);

  // Type exports (collect unique response types)
  const types = new Set<string>();
  for (const op of service.operations) {
    if (op.responseSchema?.$ref) {
      const refName = resolveRef(op.responseSchema.$ref);
      // Get the inner type if it's a response content wrapper
      if (refName.endsWith("ResponseContent")) {
        types.add(refName);
      }
    }
  }

  // Service class
  lines.push(`/**`);
  lines.push(` * ${service.description}`);
  lines.push(` */`);
  lines.push(`export class ${service.className} extends BaseService {`);

  for (const op of service.operations) {
    lines.push(``);
    lines.push(...generateMethod(op, service.name));
  }

  lines.push(`}`);

  return lines.join("\n");
}

/**
 * Generates method code for an operation.
 */
function generateMethod(op: ParsedOperation, serviceName: string): string[] {
  const lines: string[] = [];

  // Method signature
  const params = buildParams(op);
  const returnType = buildReturnType(op);

  lines.push(`  /**`);
  lines.push(`   * ${op.description.split("\n")[0]}`);
  lines.push(`   */`);
  lines.push(`  async ${op.methodName}(${params}): Promise<${returnType}> {`);

  // Request body handling
  const bodyMapping = buildBodyMapping(op);

  // Build the request call
  if (op.returnsVoid) {
    lines.push(`    await this.request(`);
  } else {
    lines.push(`    const response = await this.request(`);
  }

  lines.push(`      {`);
  lines.push(`        service: "${serviceName}",`);
  lines.push(`        operation: "${op.operationId.replace(/^(Get|List|Create|Update|Delete|Trash)/, "$1")}",`);
  lines.push(`        resourceType: "${op.resourceType}",`);
  lines.push(`        isMutation: ${op.isMutation},`);

  // Add projectId if present
  const projectParam = op.pathParams.find((p) => p.name === "projectId");
  if (projectParam) {
    lines.push(`        projectId,`);
  }

  // Add resourceId if present (first non-project path param)
  const resourceParam = op.pathParams.find(
    (p) => p.name !== "projectId" && p.name.endsWith("Id")
  );
  if (resourceParam) {
    lines.push(`        resourceId: ${resourceParam.name},`);
  }

  lines.push(`      },`);
  lines.push(`      () =>`);
  lines.push(`        this.client.${op.httpMethod}("${op.path}", {`);

  // Path and query params
  const pathParamNames = op.pathParams.map((p) => p.name);
  const hasPathParams = pathParamNames.length > 0;
  const hasQueryParams = op.queryParams.length > 0;

  const isOctetStream = op.bodyContentType === "octet-stream";
  const hasParams = hasPathParams || hasQueryParams || isOctetStream;

  if (hasParams) {
    lines.push(`          params: {`);

    if (hasPathParams) {
      lines.push(`            path: { ${pathParamNames.join(", ")} },`);
    }

    if (hasQueryParams) {
      const queryParts = op.queryParams.map((q) => {
        const camelName = toCamelCase(q.name);
        const key = q.name.includes("_") ? `"${q.name}"` : q.name;
        // Required params are direct, optional use options?.
        const value = q.required ? camelName : `options?.${camelName}`;
        return `${key}: ${value}`;
      });
      lines.push(`            query: { ${queryParts.join(", ")} },`);
    }

    // For octet-stream uploads, add Content-Type header
    if (isOctetStream) {
      lines.push(`            // eslint-disable-next-line @typescript-eslint/no-explicit-any`);
      lines.push(`            header: { "Content-Type": contentType } as any,`);
    }

    lines.push(`          },`);
  }

  // Body
  if (bodyMapping) {
    if (isOctetStream) {
      // For binary uploads, bypass JSON serialization
      lines.push(`          body: ${bodyMapping} as unknown as string,`);
      lines.push(`          // eslint-disable-next-line @typescript-eslint/no-explicit-any`);
      lines.push(`          bodySerializer: (body: unknown) => body as any,`);
    } else {
      lines.push(`          body: ${bodyMapping},`);
    }
  }

  lines.push(`        })`);
  lines.push(`    );`);

  // Return statement - return full response, let caller access wrapper properties
  if (!op.returnsVoid) {
    if (op.returnsArray) {
      lines.push(`    return response ?? [];`);
    } else {
      lines.push(`    return response;`);
    }
  }

  lines.push(`  }`);

  return lines;
}

/**
 * Builds method parameter list.
 */
function buildParams(op: ParsedOperation): string {
  const params: string[] = [];

  // Path params (except accountId)
  for (const p of op.pathParams) {
    params.push(`${p.name}: ${p.type}`);
  }

  // Request body params
  if (op.bodySchema) {
    if (op.bodyContentType === "octet-stream") {
      // File upload - body is raw binary data, plus contentType header
      const refName = op.bodySchema.$ref ? resolveRef(op.bodySchema.$ref) : null;
      if (refName) {
        params.push(`data: components["schemas"]["${refName}"]`);
      } else {
        params.push(`data: ArrayBuffer | Uint8Array | string`);
      }
      // Add contentType parameter for binary uploads
      params.push(`contentType: string`);
    } else {
      const refName = op.bodySchema.$ref ? resolveRef(op.bodySchema.$ref) : null;
      if (refName) {
        params.push(`req: components["schemas"]["${refName}"]`);
      }
    }
  }

  // Required query params as direct parameters
  const requiredQueryParams = op.queryParams.filter((q) => q.required);
  const optionalQueryParams = op.queryParams.filter((q) => !q.required);

  for (const q of requiredQueryParams) {
    params.push(`${toCamelCase(q.name)}: ${q.type}`);
  }

  // Optional query params as options object
  if (optionalQueryParams.length > 0) {
    const optionsType = optionalQueryParams
      .map((q) => `${toCamelCase(q.name)}?: ${q.type}`)
      .join("; ");
    params.push(`options?: { ${optionsType} }`);
  }

  return params.join(", ");
}

/**
 * Builds return type.
 */
function buildReturnType(op: ParsedOperation): string {
  if (op.returnsVoid) {
    return "void";
  }

  if (op.responseSchema?.$ref) {
    const refName = resolveRef(op.responseSchema.$ref);
    return `components["schemas"]["${refName}"]`;
  }

  if (op.returnsArray) {
    return "unknown[]";
  }

  return "unknown";
}

/**
 * Builds body mapping.
 */
function buildBodyMapping(op: ParsedOperation): string | null {
  if (!op.bodySchema) return null;

  if (op.bodyContentType === "octet-stream") {
    return "data";
  }

  // JSON body - pass req directly
  return "req";
}

/**
 * Converts snake_case to camelCase.
 */
function toCamelCase(str: string): string {
  return str.replace(/_([a-z])/g, (_, c) => c.toUpperCase());
}

// =============================================================================
// Main
// =============================================================================

function main() {
  // Parse arguments
  const args = process.argv.slice(2);
  let openapiPath = "../openapi.json";
  let outputDir = "src/generated/services";

  for (let i = 0; i < args.length; i++) {
    if (args[i] === "--openapi" && args[i + 1]) {
      openapiPath = args[++i];
    } else if (args[i] === "--output" && args[i + 1]) {
      outputDir = args[++i];
    }
  }

  // Resolve paths
  const resolvedOpenapiPath = path.resolve(openapiPath);
  const resolvedOutputDir = path.resolve(outputDir);

  // Read OpenAPI spec
  if (!fs.existsSync(resolvedOpenapiPath)) {
    console.error(`Error: OpenAPI file not found: ${resolvedOpenapiPath}`);
    process.exit(1);
  }

  const spec: OpenAPISpec = JSON.parse(fs.readFileSync(resolvedOpenapiPath, "utf-8"));

  // Group operations into services
  const services = groupOperations(spec);

  // Create output directory
  if (!fs.existsSync(resolvedOutputDir)) {
    fs.mkdirSync(resolvedOutputDir, { recursive: true });
  }

  // Generate service files
  const generatedFiles: string[] = [];
  for (const [name, service] of services) {
    const code = generateService(service);
    const fileName = `${toKebabCase(name)}.ts`;
    const filePath = path.join(resolvedOutputDir, fileName);
    fs.writeFileSync(filePath, code);
    generatedFiles.push(fileName);
    console.log(`Generated ${fileName} (${service.operations.length} operations)`);
  }

  // Generate index.ts barrel export
  const indexCode = generatedFiles
    .map((f) => `export * from "./${f.replace(".ts", ".js")}";`)
    .join("\n");
  fs.writeFileSync(path.join(resolvedOutputDir, "index.ts"), indexCode + "\n");
  console.log(`Generated index.ts`);

  console.log(`\nGenerated ${services.size} services with ${
    Array.from(services.values()).reduce((sum, s) => sum + s.operations.length, 0)
  } operations total.`);
}

/**
 * Converts PascalCase to kebab-case.
 */
function toKebabCase(str: string): string {
  return str
    .replace(/([a-z])([A-Z])/g, "$1-$2")
    .replace(/([A-Z]+)([A-Z][a-z])/g, "$1-$2")
    .toLowerCase();
}

main();
