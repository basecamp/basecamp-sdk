#!/usr/bin/env node
/**
 * Generates TypeScript service classes from OpenAPI spec.
 *
 * Usage: npx tsx scripts/generate-services.ts [--openapi ../openapi.json] [--output src/generated/services]
 *
 * This generator produces Go-SDK-quality output:
 * 1. Type exports for response and request types
 * 2. Documented interfaces for requests/options
 * 3. Clean method signatures with proper types
 * 4. Rich JSDoc documentation
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
  summary?: string;
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
  description?: string;
  $ref?: string;
  properties?: Record<string, Schema>;
  required?: string[];
  items?: Schema;
  "x-go-type"?: string;
}

interface ParsedOperation {
  operationId: string;
  methodName: string;
  httpMethod: "GET" | "POST" | "PUT" | "DELETE" | "PATCH";
  path: string;
  description: string;
  pathParams: PathParam[];
  queryParams: QueryParam[];
  bodySchemaRef?: string;
  bodyProperties: BodyProperty[];
  bodyRequired: boolean;
  bodyContentType?: "json" | "octet-stream";
  responseSchemaRef?: string;
  returnsArray: boolean;
  returnsVoid: boolean;
  isMutation: boolean;
  resourceType: string;
}

interface PathParam {
  name: string;
  type: string;
  description?: string;
}

interface QueryParam {
  name: string;
  type: string;
  required: boolean;
  description?: string;
}

interface BodyProperty {
  name: string;
  type: string;
  required: boolean;
  description?: string;
  formatHint?: string;
}

interface ServiceDefinition {
  name: string;
  className: string;
  description: string;
  operations: ParsedOperation[];
  types: Map<string, TypeDefinition>;
}

interface TypeDefinition {
  name: string;
  schemaRef: string;
  description?: string;
  isArray?: boolean;
}

// =============================================================================
// Configuration
// =============================================================================

/**
 * Tag to service name mapping overrides.
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
 * Service split configuration - some tags map to multiple service classes.
 */
const SERVICE_SPLITS: Record<string, Record<string, string[]>> = {
  Campfire: {
    Campfires: [
      "GetCampfire", "ListCampfires",
      "ListChatbots", "CreateChatbot", "GetChatbot", "UpdateChatbot", "DeleteChatbot",
      "ListCampfireLines", "CreateCampfireLine", "GetCampfireLine", "DeleteCampfireLine",
    ],
  },
  "Card Tables": {
    CardTables: ["GetCardTable"],
    Cards: ["GetCard", "UpdateCard", "MoveCard", "CreateCard", "ListCards"],
    CardColumns: [
      "GetCardColumn", "UpdateCardColumn", "SetCardColumnColor",
      "EnableCardColumnOnHold", "DisableCardColumnOnHold",
      "CreateCardColumn", "MoveCardColumn",
      "SubscribeToCardColumn", "UnsubscribeFromCardColumn",
    ],
    CardSteps: [
      "CreateCardStep", "UpdateCardStep", "CompleteCardStep",
      "UncompleteCardStep", "RepositionCardStep",
    ],
  },
  Files: {
    Attachments: ["CreateAttachment"],
    Uploads: ["GetUpload", "UpdateUpload", "ListUploads", "CreateUpload", "ListUploadVersions"],
    Vaults: ["GetVault", "UpdateVault", "ListVaults", "CreateVault"],
    Documents: ["GetDocument", "UpdateDocument", "ListDocuments", "CreateDocument"],
  },
  Automation: {
    Tools: ["GetTool", "UpdateTool", "DeleteTool", "CloneTool", "EnableTool", "DisableTool", "RepositionTool"],
    Recordings: ["GetRecording", "ArchiveRecording", "UnarchiveRecording", "TrashRecording", "ListRecordings"],
    Webhooks: ["ListWebhooks", "CreateWebhook", "GetWebhook", "UpdateWebhook", "DeleteWebhook"],
    Events: ["ListEvents"],
    Lineup: ["CreateLineupMarker", "UpdateLineupMarker", "DeleteLineupMarker"],
    Search: ["Search", "GetSearchMetadata"],
    Templates: [
      "ListTemplates", "CreateTemplate", "GetTemplate", "UpdateTemplate",
      "DeleteTemplate", "CreateProjectFromTemplate", "GetProjectConstruction",
    ],
    Checkins: [
      "GetQuestionnaire", "ListQuestions", "CreateQuestion", "GetQuestion",
      "UpdateQuestion", "ListAnswers", "CreateAnswer", "GetAnswer", "UpdateAnswer",
    ],
  },
  Messages: {
    Messages: ["GetMessage", "UpdateMessage", "CreateMessage", "ListMessages", "PinMessage", "UnpinMessage"],
    MessageBoards: ["GetMessageBoard"],
    MessageTypes: ["ListMessageTypes", "CreateMessageType", "GetMessageType", "UpdateMessageType", "DeleteMessageType"],
    Comments: ["GetComment", "UpdateComment", "ListComments", "CreateComment"],
  },
  People: {
    People: ["GetMyProfile", "ListPeople", "GetPerson", "ListProjectPeople", "UpdateProjectAccess", "ListPingablePeople", "ListAssignablePeople"],
    Subscriptions: ["GetSubscription", "Subscribe", "Unsubscribe", "UpdateSubscription"],
  },
  Schedule: {
    Schedules: [
      "GetSchedule", "UpdateScheduleSettings", "ListScheduleEntries",
      "CreateScheduleEntry", "GetScheduleEntry", "UpdateScheduleEntry", "GetScheduleEntryOccurrence",
    ],
    Timesheets: ["GetRecordingTimesheet", "GetProjectTimesheet", "GetTimesheetReport"],
  },
  ClientFeatures: {
    ClientApprovals: ["ListClientApprovals", "GetClientApproval"],
    ClientCorrespondences: ["ListClientCorrespondences", "GetClientCorrespondence"],
    ClientReplies: ["ListClientReplies", "GetClientReply"],
    ClientVisibility: ["SetClientVisibility"],
  },
  Todos: {
    Todos: ["ListTodos", "CreateTodo", "GetTodo", "UpdateTodo", "CompleteTodo", "UncompleteTodo", "TrashTodo"],
    Todolists: ["GetTodolistOrGroup", "UpdateTodolistOrGroup", "ListTodolists", "CreateTodolist"],
    Todosets: ["GetTodoset"],
    TodolistGroups: ["ListTodolistGroups", "CreateTodolistGroup", "RepositionTodolistGroup"],
  },
  Untagged: {
    Timeline: ["GetProjectTimeline"],
    Reports: ["GetProgressReport", "GetUpcomingSchedule", "GetAssignedTodos", "GetOverdueTodos", "GetPersonProgress"],
    Checkins: [
      "GetQuestionReminders", "ListQuestionAnswerers", "GetAnswersByPerson",
      "UpdateQuestionNotificationSettings", "PauseQuestion", "ResumeQuestion",
    ],
    Todos: ["RepositionTodo"],
    People: ["ListAssignablePeople"],
    CardColumns: ["SubscribeToCardColumn", "UnsubscribeFromCardColumn"],
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
  GetMyProfile: "me",
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
  GetSearchMetadata: "metadata",
  Search: "search",
  CreateProjectFromTemplate: "createProject",
  GetProjectConstruction: "getConstruction",
  GetRecordingTimesheet: "forRecording",
  GetProjectTimesheet: "forProject",
  GetTimesheetReport: "report",
  GetProgressReport: "progress",
  GetUpcomingSchedule: "upcoming",
  GetAssignedTodos: "assigned",
  GetOverdueTodos: "overdue",
  GetPersonProgress: "personProgress",
  SubscribeToCardColumn: "subscribeToColumn",
  UnsubscribeFromCardColumn: "unsubscribeFromColumn",
  SetClientVisibility: "setVisibility",
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
  GetForward: "get",
  ListForwards: "list",
  GetForwardReply: "getReply",
  ListForwardReplies: "listReplies",
  CreateForwardReply: "createReply",
  GetInbox: "getInbox",
  GetUpload: "get",
  UpdateUpload: "update",
  ListUploads: "list",
  CreateUpload: "create",
  ListUploadVersions: "listVersions",
  GetMessage: "get",
  UpdateMessage: "update",
  CreateMessage: "create",
  ListMessages: "list",
  PinMessage: "pin",
  UnpinMessage: "unpin",
  GetMessageBoard: "get",
  GetMessageType: "get",
  UpdateMessageType: "update",
  CreateMessageType: "create",
  ListMessageTypes: "list",
  DeleteMessageType: "delete",
  GetComment: "get",
  UpdateComment: "update",
  CreateComment: "create",
  ListComments: "list",
  ListProjectPeople: "listForProject",
  ListPingablePeople: "listPingable",
  ListAssignablePeople: "listAssignable",
  GetSchedule: "get",
  UpdateScheduleSettings: "updateSettings",
  GetScheduleEntry: "getEntry",
  UpdateScheduleEntry: "updateEntry",
  CreateScheduleEntry: "createEntry",
  ListScheduleEntries: "listEntries",
  GetScheduleEntryOccurrence: "getEntryOccurrence",
};

/**
 * Maps actual OpenAPI schema names to friendly type names.
 * Format: SchemaName -> [TypeAlias, kind]
 * These are the actual entity schemas, not ResponseContent wrappers.
 */
const TYPE_ALIASES: Record<string, [string, "response" | "request" | "entity"]> = {
  // Core entity types (matching actual OpenAPI schema names)
  Todo: ["Todo", "entity"],
  Person: ["Person", "entity"],
  Project: ["Project", "entity"],
  Message: ["Message", "entity"],
  Comment: ["Comment", "entity"],
  Card: ["Card", "entity"],
  CardTable: ["CardTable", "entity"],
  CardColumn: ["CardColumn", "entity"],
  CardStep: ["CardStep", "entity"],
  Campfire: ["Campfire", "entity"],
  CampfireLine: ["CampfireLine", "entity"],
  Chatbot: ["Chatbot", "entity"],
  Webhook: ["Webhook", "entity"],
  Vault: ["Vault", "entity"],
  Document: ["Document", "entity"],
  Upload: ["Upload", "entity"],
  Schedule: ["Schedule", "entity"],
  ScheduleEntry: ["ScheduleEntry", "entity"],
  Recording: ["Recording", "entity"],
  Template: ["Template", "entity"],
  Todolist: ["Todolist", "entity"],
  Todoset: ["Todoset", "entity"],
  TodolistGroup: ["TodolistGroup", "entity"],
  Questionnaire: ["Questionnaire", "entity"],
  Question: ["Question", "entity"],
  QuestionAnswer: ["Answer", "entity"], // Schema is QuestionAnswer, type alias is Answer
  Subscription: ["Subscription", "entity"],
  Forward: ["Forward", "entity"],
  ForwardReply: ["ForwardReply", "entity"],
  Inbox: ["Inbox", "entity"],
  MessageBoard: ["MessageBoard", "entity"],
  MessageType: ["MessageType", "entity"],
  Event: ["Event", "entity"],
  Tool: ["Tool", "entity"],
  LineupMarker: ["LineupMarker", "entity"],
  ClientApproval: ["ClientApproval", "entity"],
  ClientCorrespondence: ["ClientCorrespondence", "entity"],
  ClientReply: ["ClientReply", "entity"],
};

// =============================================================================
// Schema Utilities
// =============================================================================

let globalSchemas: Record<string, Schema> = {};

function setSchemas(schemas: Record<string, Schema>) {
  globalSchemas = schemas;
}

function resolveRef(ref: string): string {
  return ref.split("/").pop() || "";
}

function resolveSchema(schemaOrRef: Schema): Schema | undefined {
  if (schemaOrRef.$ref) {
    const refName = resolveRef(schemaOrRef.$ref);
    return globalSchemas[refName];
  }
  return schemaOrRef;
}

function getSchemaProperties(schemaRef: string): { properties: Record<string, Schema>; required: string[] } {
  const schema = globalSchemas[schemaRef];
  if (!schema) return { properties: {}, required: [] };
  return {
    properties: schema.properties || {},
    required: schema.required || [],
  };
}

function schemaToTsType(schema: Schema, forInterface = false): string {
  if (schema.$ref) {
    const refName = resolveRef(schema.$ref);
    // For interface properties, use full component reference since we don't import all types
    return forInterface ? `components["schemas"]["${refName}"]` : refName;
  }
  switch (schema.type) {
    case "integer":
      return "number";
    case "boolean":
      return "boolean";
    case "array":
      return schema.items ? `${schemaToTsType(schema.items, forInterface)}[]` : "unknown[]";
    case "object":
      return "Record<string, unknown>";
    default:
      return "string";
  }
}

function getFormatHint(schema: Schema): string | undefined {
  if (schema["x-go-type"] === "types.Date") return "YYYY-MM-DD";
  if (schema["x-go-type"] === "time.Time" || schema["x-go-type"] === "types.DateTime") {
    return "RFC3339 (e.g., 2024-12-15T09:00:00Z)";
  }
  if (schema.format === "date") return "YYYY-MM-DD";
  if (schema.format === "date-time") return "RFC3339";
  return undefined;
}

// =============================================================================
// Parsing Functions
// =============================================================================

function extractMethodName(operationId: string): string {
  if (METHOD_NAME_OVERRIDES[operationId]) {
    return METHOD_NAME_OVERRIDES[operationId];
  }

  for (const { prefix, method } of VERB_PATTERNS) {
    if (operationId.startsWith(prefix)) {
      const remainder = operationId.slice(prefix.length);
      if (!remainder) return method;
      const resource = remainder.charAt(0).toLowerCase() + remainder.slice(1);
      if (isSimpleResource(resource)) return method;
      return method === "get" ? resource : method + remainder;
    }
  }

  return operationId.charAt(0).toLowerCase() + operationId.slice(1);
}

function isSimpleResource(resource: string): boolean {
  const simpleResources = [
    "todo", "todos", "todolist", "todolists", "todoset",
    "message", "messages", "comment", "comments",
    "card", "cards", "cardtable", "cardcolumn", "cardstep", "column", "step",
    "project", "projects", "person", "people",
    "campfire", "campfires", "chatbot", "chatbots",
    "webhook", "webhooks", "vault", "vaults", "document", "documents",
    "upload", "uploads", "schedule", "scheduleentry", "scheduleentries",
    "event", "events", "recording", "recordings", "template", "templates",
    "attachment", "question", "questions", "answer", "answers", "questionnaire",
    "subscription", "forward", "forwards", "inbox", "messageboard",
    "messagetype", "messagetypes", "tool", "lineupmarker",
    "clientapproval", "clientapprovals", "clientcorrespondence", "clientcorrespondences",
    "clientreply", "clientreplies", "forwardreply", "forwardreplies",
    "campfireline", "campfirelines", "todolistgroup", "todolistgroups",
    "todolistorgroup", "uploadversions",
  ];
  return simpleResources.includes(resource.toLowerCase());
}

function extractResourceType(operationId: string): string {
  for (const { prefix } of VERB_PATTERNS) {
    if (operationId.startsWith(prefix)) {
      const remainder = operationId.slice(prefix.length);
      if (!remainder) return "resource";
      const snakeCase = remainder
        .replace(/([A-Z])/g, "_$1")
        .toLowerCase()
        .replace(/^_/, "");
      return snakeCase.replace(/([^s])s$/, "$1");
    }
  }
  return "resource";
}

function convertPath(path: string): string {
  return path.replace(/^\/{accountId}/, "");
}

function isVoidResponse(responses: Record<string, Response> | undefined): boolean {
  if (!responses) return true;
  const successResponse = responses["200"] || responses["201"] || responses["204"];
  if (!successResponse) return true;
  return !successResponse.content?.["application/json"];
}

function parseOperation(
  path: string,
  method: string,
  operation: Operation,
): ParsedOperation {
  const httpMethod = method.toUpperCase() as "GET" | "POST" | "PUT" | "DELETE" | "PATCH";
  const operationId = operation.operationId;
  const methodName = extractMethodName(operationId);
  const description = operation.description || operation.summary || `${methodName} operation`;

  // Path parameters
  const pathParams: PathParam[] = (operation.parameters || [])
    .filter((p) => p.in === "path" && p.name !== "accountId")
    .map((p) => ({
      name: p.name,
      type: p.schema.type === "integer" ? "number" : "string",
      description: p.description,
    }));

  // Query parameters
  const queryParams: QueryParam[] = (operation.parameters || [])
    .filter((p) => p.in === "query")
    .map((p) => ({
      name: p.name,
      type: schemaToTsType(p.schema),
      required: p.required || false,
      description: p.description,
    }));

  // Request body
  let bodySchemaRef: string | undefined;
  let bodyProperties: BodyProperty[] = [];
  let bodyRequired = false;
  let bodyContentType: "json" | "octet-stream" | undefined;

  if (operation.requestBody?.content?.["application/json"]?.schema) {
    const schema = operation.requestBody.content["application/json"].schema;
    bodyRequired = operation.requestBody.required || false;
    bodyContentType = "json";
    if (schema.$ref) {
      bodySchemaRef = resolveRef(schema.$ref);
      const { properties, required } = getSchemaProperties(bodySchemaRef);
      bodyProperties = Object.entries(properties).map(([name, prop]) => ({
        name,
        type: schemaToTsType(prop, true), // forInterface=true to use full schema refs
        required: required.includes(name),
        description: prop.description,
        formatHint: getFormatHint(prop),
      }));
    }
  } else if (operation.requestBody?.content?.["application/octet-stream"]?.schema) {
    bodyRequired = operation.requestBody.required || false;
    bodyContentType = "octet-stream";
  }

  // Response
  let responseSchemaRef: string | undefined;
  let returnsArray = false;
  const successResponse = operation.responses?.["200"] || operation.responses?.["201"];
  if (successResponse?.content?.["application/json"]?.schema) {
    const schema = successResponse.content["application/json"].schema;
    if (schema.$ref) {
      responseSchemaRef = resolveRef(schema.$ref);
      // Check if the referenced schema is an array type
      const resolvedSchema = globalSchemas[responseSchemaRef];
      if (resolvedSchema?.type === "array") {
        returnsArray = true;
      }
    }
    if (schema.type === "array") {
      returnsArray = true;
    }
  }

  const returnsVoid = isVoidResponse(operation.responses);
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
    bodySchemaRef,
    bodyProperties,
    bodyRequired,
    bodyContentType,
    responseSchemaRef,
    returnsArray,
    returnsVoid,
    isMutation,
    resourceType,
  };
}

function groupOperations(spec: OpenAPISpec): Map<string, ServiceDefinition> {
  const services = new Map<string, ServiceDefinition>();

  for (const [path, pathItem] of Object.entries(spec.paths)) {
    for (const method of ["get", "post", "put", "patch", "delete"]) {
      const operation = pathItem[method];
      if (!operation) continue;

      const tag = operation.tags?.[0] || "Untagged";
      const parsed = parseOperation(path, method, operation);

      // Determine service
      let serviceName: string;
      if (SERVICE_SPLITS[tag]) {
        let found = false;
        for (const [svc, opIds] of Object.entries(SERVICE_SPLITS[tag])) {
          if (opIds.includes(operation.operationId)) {
            serviceName = svc;
            found = true;
            break;
          }
        }
        if (!found) {
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
          types: new Map(),
        });
      }

      const service = services.get(serviceName)!;
      service.operations.push(parsed);

      // Collect types used by this service
      if (parsed.responseSchemaRef) {
        const entityName = getEntityTypeName(parsed.responseSchemaRef);
        if (entityName) {
          service.types.set(entityName, {
            name: entityName,
            schemaRef: parsed.responseSchemaRef,
            isArray: parsed.returnsArray,
          });
        }
      }
    }
  }

  return services;
}

function getEntityTypeName(schemaRef: string): string | null {
  // Direct entity reference - check if schema is in TYPE_ALIASES
  if (TYPE_ALIASES[schemaRef]) {
    return TYPE_ALIASES[schemaRef][0];
  }

  // For ResponseContent types, resolve to the underlying entity schema
  const entitySchema = findUnderlyingEntitySchema(schemaRef);
  if (entitySchema && TYPE_ALIASES[entitySchema]) {
    return TYPE_ALIASES[entitySchema][0];
  }

  return null;
}

// =============================================================================
// Code Generation
// =============================================================================

function generateService(service: ServiceDefinition): string {
  const lines: string[] = [];
  const serviceName = service.name;

  // File header
  lines.push(`/**`);
  lines.push(` * ${serviceName} service for the Basecamp API.`);
  lines.push(` *`);
  lines.push(` * @generated from OpenAPI spec - do not edit directly`);
  lines.push(` */`);
  lines.push(``);
  lines.push(`import { BaseService } from "../../services/base.js";`);
  lines.push(`import type { components } from "../schema.js";`);
  lines.push(``);

  // Type exports
  lines.push(`// =============================================================================`);
  lines.push(`// Types`);
  lines.push(`// =============================================================================`);
  lines.push(``);

  // Collect all unique types needed
  const typeExports = collectTypeExports(service);
  for (const typeExport of typeExports) {
    lines.push(typeExport);
  }

  // Request/Options interfaces
  const requestInterfaces = generateRequestInterfaces(service);
  if (requestInterfaces.length > 0) {
    lines.push(``);
    lines.push(...requestInterfaces);
  }

  // Service class
  lines.push(``);
  lines.push(`// =============================================================================`);
  lines.push(`// Service`);
  lines.push(`// =============================================================================`);
  lines.push(``);
  lines.push(`/**`);
  lines.push(` * Service for ${serviceName} operations.`);
  lines.push(` */`);
  lines.push(`export class ${service.className} extends BaseService {`);

  for (const op of service.operations) {
    lines.push(``);
    lines.push(...generateMethod(op, serviceName));
  }

  lines.push(`}`);

  return lines.join("\n");
}

function collectTypeExports(service: ServiceDefinition): string[] {
  const exports: string[] = [];
  const added = new Set<string>();

  // Collect response types
  for (const op of service.operations) {
    if (op.responseSchemaRef && !op.returnsVoid) {
      // Find the underlying entity schema (e.g., "Todo" from "GetTodoResponseContent")
      const entitySchema = findUnderlyingEntitySchema(op.responseSchemaRef);
      if (entitySchema && TYPE_ALIASES[entitySchema]) {
        const [typeName] = TYPE_ALIASES[entitySchema];
        if (!added.has(typeName)) {
          exports.push(`/** ${typeName} entity from the Basecamp API. */`);
          exports.push(`export type ${typeName} = components["schemas"]["${entitySchema}"];`);
          added.add(typeName);
        }
      }
    }
  }

  return exports;
}

function findUnderlyingEntitySchema(responseSchemaRef: string): string | null {
  // ResponseContent types often alias entity types
  const schema = globalSchemas[responseSchemaRef];
  if (!schema) return null;

  // Check if it's a direct $ref to a known entity
  if (schema.$ref) {
    const refName = resolveRef(schema.$ref);
    // Only return if it's a known entity type
    if (TYPE_ALIASES[refName]) {
      return refName;
    }
  }

  // If it's an array, get item type (only if it's a known entity)
  if (schema.type === "array" && schema.items?.$ref) {
    const refName = resolveRef(schema.items.$ref);
    if (TYPE_ALIASES[refName]) {
      return refName;
    }
  }

  // Don't fall back to response schema - it may not be a true entity type
  return null;
}

function generateRequestInterfaces(service: ServiceDefinition): string[] {
  const lines: string[] = [];
  const generated = new Set<string>();

  for (const op of service.operations) {
    // Generate request interfaces for create/update operations
    if (op.bodySchemaRef && op.bodyProperties.length > 0) {
      const interfaceName = `${capitalize(op.methodName)}${capitalize(service.name.replace(/s$/, ""))}Request`;
      if (generated.has(interfaceName)) continue;
      generated.add(interfaceName);

      lines.push(`/**`);
      lines.push(` * Request parameters for ${op.methodName}.`);
      lines.push(` */`);
      lines.push(`export interface ${interfaceName} {`);

      for (const prop of op.bodyProperties) {
        const optional = prop.required ? "" : "?";
        const desc = prop.description || toHumanReadable(prop.name);
        const format = prop.formatHint ? ` (${prop.formatHint})` : "";
        lines.push(`  /** ${desc}${format} */`);
        lines.push(`  ${toCamelCase(prop.name)}${optional}: ${mapPropertyType(prop.type)};`);
      }

      lines.push(`}`);
      lines.push(``);
    }

    // Generate options interfaces for query params
    const optionalQueryParams = op.queryParams.filter((q) => !q.required);
    if (optionalQueryParams.length > 0) {
      const interfaceName = `${capitalize(op.methodName)}${capitalize(service.name.replace(/s$/, ""))}Options`;
      if (generated.has(interfaceName)) continue;
      generated.add(interfaceName);

      lines.push(`/**`);
      lines.push(` * Options for ${op.methodName}.`);
      lines.push(` */`);
      lines.push(`export interface ${interfaceName} {`);

      for (const param of optionalQueryParams) {
        const desc = param.description || toHumanReadable(param.name);
        lines.push(`  /** ${desc} */`);
        lines.push(`  ${toCamelCase(param.name)}?: ${param.type};`);
      }

      lines.push(`}`);
      lines.push(``);
    }
  }

  return lines;
}

function mapPropertyType(type: string): string {
  // Map schema types to cleaner TypeScript types
  switch (type) {
    case "Array":
      return "number[]"; // Usually IDs
    default:
      return type;
  }
}

function generateMethod(op: ParsedOperation, serviceName: string): string[] {
  const lines: string[] = [];
  const resourceName = serviceName.replace(/s$/, "");

  // Build param string and types
  const { paramString, hasOptions, hasRequest, requestInterfaceName, optionsInterfaceName } = buildMethodSignature(op, resourceName);

  // Return type
  const returnType = buildReturnType(op, serviceName);

  // JSDoc
  lines.push(`  /**`);
  lines.push(`   * ${op.description.split("\n")[0]}`);

  // @param tags
  for (const p of op.pathParams) {
    lines.push(`   * @param ${p.name} - The ${toHumanReadable(p.name)}`);
  }
  if (hasRequest) {
    lines.push(`   * @param req - Request parameters`);
  }
  if (op.bodyContentType === "octet-stream") {
    lines.push(`   * @param data - Binary file data to upload`);
    lines.push(`   * @param contentType - MIME type of the file (e.g., "image/png", "application/pdf")`);
  }
  // Required query params
  const requiredQueryParams = op.queryParams.filter((q) => q.required);
  for (const q of requiredQueryParams) {
    const desc = q.description || toHumanReadable(q.name);
    lines.push(`   * @param ${toCamelCase(q.name)} - ${desc}`);
  }
  if (hasOptions) {
    lines.push(`   * @param options - Optional parameters`);
  }

  // @returns
  if (op.returnsVoid) {
    lines.push(`   * @returns void`);
  } else if (op.returnsArray) {
    const entityType = getEntityTypeName(op.responseSchemaRef || "");
    lines.push(`   * @returns Array of ${entityType || "results"}`);
  } else {
    const entityType = getEntityTypeName(op.responseSchemaRef || "");
    lines.push(`   * @returns The ${entityType || op.resourceType}`);
  }

  // @example for create methods
  if (op.methodName.startsWith("create") || op.methodName === "create") {
    lines.push(`   *`);
    lines.push(`   * @example`);
    lines.push(`   * \`\`\`ts`);
    const exampleArgs = generateExampleArgs(op, hasRequest);
    lines.push(`   * const result = await client.${camelCase(serviceName)}.${op.methodName}(${exampleArgs});`);
    lines.push(`   * \`\`\``);
  }

  lines.push(`   */`);

  // Method signature
  lines.push(`  async ${op.methodName}(${paramString}): Promise<${returnType}> {`);

  // Method body
  if (op.returnsVoid) {
    lines.push(`    await this.request(`);
  } else {
    lines.push(`    const response = await this.request(`);
  }

  lines.push(`      {`);
  lines.push(`        service: "${serviceName}",`);
  lines.push(`        operation: "${op.operationId}",`);
  lines.push(`        resourceType: "${op.resourceType}",`);
  lines.push(`        isMutation: ${op.isMutation},`);

  const projectParam = op.pathParams.find((p) => p.name === "projectId");
  if (projectParam) {
    lines.push(`        projectId,`);
  }

  const resourceParam = op.pathParams.find((p) => p.name !== "projectId" && p.name.endsWith("Id"));
  if (resourceParam) {
    lines.push(`        resourceId: ${resourceParam.name},`);
  }

  lines.push(`      },`);
  lines.push(`      () =>`);
  lines.push(`        this.client.${op.httpMethod}("${op.path}", {`);

  // Params object
  const pathParamNames = op.pathParams.map((p) => p.name);
  const hasPathParams = pathParamNames.length > 0;
  const hasQueryParams = op.queryParams.length > 0;
  const isOctetStream = op.bodyContentType === "octet-stream";

  if (hasPathParams || hasQueryParams || isOctetStream) {
    lines.push(`          params: {`);

    if (hasPathParams) {
      lines.push(`            path: { ${pathParamNames.join(", ")} },`);
    }

    if (hasQueryParams) {
      const queryParts = op.queryParams.map((q) => {
        const camelName = toCamelCase(q.name);
        const key = q.name.includes("_") ? `"${q.name}"` : q.name;
        const value = q.required ? camelName : `options?.${camelName}`;
        return `${key}: ${value}`;
      });
      lines.push(`            query: { ${queryParts.join(", ")} },`);
    }

    if (isOctetStream) {
      lines.push(`            // eslint-disable-next-line @typescript-eslint/no-explicit-any`);
      lines.push(`            header: { "Content-Type": contentType } as any,`);
    }

    lines.push(`          },`);
  }

  // Body
  if (op.bodySchemaRef && op.bodyContentType === "json") {
    // Convert camelCase request to snake_case API body
    lines.push(`          body: ${buildBodyMapping(op)},`);
  } else if (isOctetStream) {
    lines.push(`          body: data as unknown as string,`);
    lines.push(`          // eslint-disable-next-line @typescript-eslint/no-explicit-any`);
    lines.push(`          bodySerializer: (body: unknown) => body as any,`);
  }

  lines.push(`        })`);
  lines.push(`    );`);

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

function buildMethodSignature(op: ParsedOperation, resourceName: string): {
  paramString: string;
  hasOptions: boolean;
  hasRequest: boolean;
  requestInterfaceName: string;
  optionsInterfaceName: string;
} {
  const params: string[] = [];
  let hasOptions = false;
  let hasRequest = false;
  const requestInterfaceName = `${capitalize(op.methodName)}${capitalize(resourceName)}Request`;
  const optionsInterfaceName = `${capitalize(op.methodName)}${capitalize(resourceName)}Options`;

  // Path params
  for (const p of op.pathParams) {
    params.push(`${p.name}: ${p.type}`);
  }

  // Body params (use generated interface)
  if (op.bodySchemaRef && op.bodyProperties.length > 0 && op.bodyContentType === "json") {
    params.push(`req: ${requestInterfaceName}`);
    hasRequest = true;
  }

  // Binary upload
  if (op.bodyContentType === "octet-stream") {
    params.push(`data: ArrayBuffer | Uint8Array | string`);
    params.push(`contentType: string`);
  }

  // Query params (required first, then options)
  const requiredQueryParams = op.queryParams.filter((q) => q.required);
  const optionalQueryParams = op.queryParams.filter((q) => !q.required);

  for (const q of requiredQueryParams) {
    params.push(`${toCamelCase(q.name)}: ${q.type}`);
  }

  if (optionalQueryParams.length > 0) {
    params.push(`options?: ${optionsInterfaceName}`);
    hasOptions = true;
  }

  return {
    paramString: params.join(", "),
    hasOptions,
    hasRequest,
    requestInterfaceName,
    optionsInterfaceName,
  };
}

function buildReturnType(op: ParsedOperation, serviceName: string): string {
  if (op.returnsVoid) return "void";

  // Try to get a friendly type name
  if (op.responseSchemaRef) {
    const entityName = getEntityTypeName(op.responseSchemaRef);
    if (entityName) {
      return op.returnsArray ? `${entityName}[]` : entityName;
    }
    // Fallback to schema ref
    return `components["schemas"]["${op.responseSchemaRef}"]`;
  }

  if (op.returnsArray) return "unknown[]";
  return "unknown";
}

function buildBodyMapping(op: ParsedOperation): string {
  if (!op.bodyProperties.length) return "req";

  // Check if any field names differ between camelCase and snake_case
  const needsMapping = op.bodyProperties.some((p) => toCamelCase(p.name) !== p.name);

  // If all fields have same name in camelCase (no underscore conversion needed), just cast req
  if (!needsMapping) return "req as any";

  // Map camelCase request fields to snake_case API fields
  const mappings = op.bodyProperties.map((prop) => {
    const camelName = toCamelCase(prop.name);
    // Always reference from req, and use snake_case key if different
    if (camelName === prop.name) {
      return `${prop.name}: req.${camelName}`;
    }
    return `${prop.name}: req.${camelName}`;
  });

  return `{\n            ${mappings.join(",\n            ")},\n          }`;
}

function generateExampleArgs(op: ParsedOperation, hasRequest: boolean): string {
  const args: string[] = [];

  // Path params
  for (const p of op.pathParams) {
    args.push(p.type === "number" ? "123" : '"example"');
  }

  // Request body (JSON)
  if (hasRequest) {
    args.push("{ ... }");
  }

  // Binary upload
  if (op.bodyContentType === "octet-stream") {
    args.push("fileData");
    args.push('"image/png"');
  }

  // Required query params
  const requiredQueryParams = op.queryParams.filter((q) => q.required);
  for (const q of requiredQueryParams) {
    args.push(q.type === "number" ? "123" : `"${q.name}"`);
  }

  return args.join(", ");
}

// =============================================================================
// Utility Functions
// =============================================================================

function toCamelCase(str: string): string {
  return str.replace(/_([a-z])/g, (_, c) => c.toUpperCase());
}

function camelCase(str: string): string {
  return str.charAt(0).toLowerCase() + str.slice(1);
}

function capitalize(str: string): string {
  return str.charAt(0).toUpperCase() + str.slice(1);
}

function toHumanReadable(str: string): string {
  if (str.endsWith("Id")) {
    return str.slice(0, -2).replace(/([a-z])([A-Z])/g, "$1 $2").toLowerCase() + " ID";
  }
  return str.replace(/_/g, " ").replace(/([a-z])([A-Z])/g, "$1 $2").toLowerCase();
}

function toKebabCase(str: string): string {
  return str
    .replace(/([a-z])([A-Z])/g, "$1-$2")
    .replace(/([A-Z]+)([A-Z][a-z])/g, "$1-$2")
    .toLowerCase();
}

// =============================================================================
// Main
// =============================================================================

function main() {
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

  const resolvedOpenapiPath = path.resolve(openapiPath);
  const resolvedOutputDir = path.resolve(outputDir);

  if (!fs.existsSync(resolvedOpenapiPath)) {
    console.error(`Error: OpenAPI file not found: ${resolvedOpenapiPath}`);
    process.exit(1);
  }

  const spec: OpenAPISpec = JSON.parse(fs.readFileSync(resolvedOpenapiPath, "utf-8"));
  setSchemas(spec.components.schemas);

  const services = groupOperations(spec);

  if (!fs.existsSync(resolvedOutputDir)) {
    fs.mkdirSync(resolvedOutputDir, { recursive: true });
  }

  const generatedFiles: string[] = [];
  for (const [name, service] of services) {
    const code = generateService(service);
    const fileName = `${toKebabCase(name)}.ts`;
    const filePath = path.join(resolvedOutputDir, fileName);
    fs.writeFileSync(filePath, code);
    generatedFiles.push(fileName);
    console.log(`Generated ${fileName} (${service.operations.length} operations)`);
  }

  // Generate index.ts - only export service classes to avoid duplicate type exports
  // Entity types are available via each service file or directly from schema.js
  const indexLines: string[] = [];
  for (const [name, service] of services) {
    const fileName = toKebabCase(name);
    indexLines.push(`export { ${service.className} } from "./${fileName}.js";`);
  }
  fs.writeFileSync(path.join(resolvedOutputDir, "index.ts"), indexLines.join("\n") + "\n");
  console.log(`Generated index.ts`);

  console.log(`\nGenerated ${services.size} services with ${
    Array.from(services.values()).reduce((sum, s) => sum + s.operations.length, 0)
  } operations total.`);
}

main();
