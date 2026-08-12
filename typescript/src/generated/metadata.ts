// Generated from OpenAPI x-basecamp-* extensions. Do not edit by hand.

export interface RetryConfig {
  maxAttempts: number;
  baseDelayMs: number;
  backoff: "exponential" | "linear" | "constant";
  retryOn: number[];
}

export interface PaginationConfig {
  style: "link" | "cursor" | "page";
  pageParam?: string;
  totalCountHeader?: string;
  maxPageSize?: number;
  key?: string;
}

export interface IdempotentConfig {
  keySupported?: boolean;
  keyHeader?: string;
  natural?: boolean;
}

export interface OperationMetadata {
  retry?: RetryConfig;
  pagination?: PaginationConfig;
  idempotent?: IdempotentConfig;
}

export interface MetadataOutput {
  $schema: string;
  version: string;
  generated: string;
  operations: Record<string, OperationMetadata>;
}

const metadata: MetadataOutput = {
  "$schema": "https://basecamp.com/schemas/sdk-metadata.json",
  "version": "1.0.0",
  "generated": "2026-08-12T04:40:53.269Z",
  "operations": {
    "GetAccount": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateAccountLogo": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "RemoveAccountLogo": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "UpdateAccountName": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "CreateAttachment": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 2000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetBoost": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "DeleteBoost": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "SetCardColumnColor": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "EnableCardColumnOnHold": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "DisableCardColumnOnHold": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "UpdateWormhole": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "DeleteWormhole": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "CreateWormhole": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ListMessageTypes": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateMessageType": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetMessageType": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateMessageType": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "DeleteMessageType": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ListChatbots": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateChatbot": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetChatbot": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateChatbot": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "DeleteChatbot": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ListClientApprovals": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "ListClientCorrespondences": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "ListClientReplies": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "GetClientReply": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "CreateTool": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "CreateTodosetTodo": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "CreateCloudFile": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "CreateGoogleDocument": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ListWebhooks": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateWebhook": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetCalendar": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateCalendar": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetCard": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateCard": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "MoveCard": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "RepositionCardStep": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "CreateCardStep": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetCardColumn": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateCardColumn": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ListCards": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateCard": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "SubscribeToCardColumn": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "UnsubscribeFromCardColumn": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetCardStep": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateCardStep": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "SetCardStepCompletion": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetCardTable": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "CreateCardColumn": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "MoveCardColumn": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetEverythingCompletedCards": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 5
      }
    },
    "GetEverythingNoDueDateCards": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 5
      }
    },
    "GetEverythingNotNowCards": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 5
      }
    },
    "GetEverythingOpenCards": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 5
      }
    },
    "GetEverythingOverdueCards": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetEverythingUnassignedCards": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 5
      }
    },
    "ListCampfires": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "GetCampfire": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ListCampfireLines": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateCampfireLine": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetCampfireLine": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateCampfireLine": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "DeleteCampfireLine": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ListCampfireUploads": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateCampfireUpload": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 2000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetEverythingCheckins": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "ListPingablePeople": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "GetClientApproval": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetClientCorrespondence": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetCloudFile": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateCloudFile": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetEverythingComments": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "GetComment": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateComment": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetTool": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateTool": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "DeleteTool": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetDocument": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ReplaceDocument": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetEverythingFiles": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "GetEverythingForwards": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "GetGaugeNeedle": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateGaugeNeedle": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "DestroyGaugeNeedle": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetGoogleDocument": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateGoogleDocument": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetForward": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ListForwardReplies": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "GetForwardReply": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetInbox": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ListForwards": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "ListLineupMarkers": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "CreateLineupMarker": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateLineupMarker": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "DeleteLineupMarker": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetMessageBoard": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ListMessages": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateMessage": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetEverythingMessages": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "GetMessage": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateMessage": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetMyAssignments": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetMyCompletedAssignments": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetMyDueAssignments": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ListMyBookmarks": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "ListMyDrafts": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "GetMyNote": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateMyNote": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetMyPreferences": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateMyPreferences": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "PrioritizeAssignment": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "DeprioritizeAssignment": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ReorderUpNext": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetMyProfile": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateMyProfile": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetQuestionReminders": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "maxPageSize": 50
      }
    },
    "GetMyNotifications": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetBubbleUps": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "MarkAsRead": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ListPeople": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "GetPerson": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetOutOfOffice": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "EnableOutOfOffice": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "DisableOutOfOffice": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ListProjects": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateProject": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ListRecordings": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "GetProject": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateProject": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "TrashProject": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ToggleGauge": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ListGaugeNeedles": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateGaugeNeedle": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ListProjectPeople": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "UpdateProjectAccess": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "UnarchiveProject": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ArchiveProject": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetProjectTimeline": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "GetProjectTimesheet": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "GetAnswer": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateAnswer": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetQuestionnaire": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ListQuestions": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateQuestion": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetQuestion": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateQuestion": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ListAnswers": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateAnswer": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ListQuestionAnswerers": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "maxPageSize": 50
      }
    },
    "GetAnswersByPerson": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "maxPageSize": 50
      }
    },
    "UpdateQuestionNotificationSettings": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "PauseQuestion": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ResumeQuestion": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "PinMessage": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UnpinMessage": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetBookmark": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "CreateBookmark": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "DeleteBookmark": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ListRecordingBoosts": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateRecordingBoost": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "SetClientVisibility": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ListComments": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateComment": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ListEvents": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "ListEventBoosts": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateEventBoost": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UnarchiveRecording": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ArchiveRecording": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "TrashRecording": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetSubscription": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "Subscribe": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "UpdateSubscription": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "Unsubscribe": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetRecordingTimesheet": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateTimesheetEntry": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "EnableTool": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "RepositionTool": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "DisableTool": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ListGauges": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "GetProgressReport": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "GetUpcomingSchedule": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetTimesheetReport": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ListAssignablePeople": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetAssignedTodos": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetOverdueTodos": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetPersonProgress": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50,
        "key": "events"
      }
    },
    "GetScheduleEntry": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ReplaceScheduleEntry": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetScheduleEntryOccurrence": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetSchedule": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateScheduleSettings": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ListScheduleEntries": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateScheduleEntry": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "Search": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "GetSearchMetadata": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ListFolders": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "CreateFolder": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetFolder": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateFolder": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "DeleteFolder": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ListTemplates": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateTemplate": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetTemplate": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateTemplate": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "DeleteTemplate": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "CreateProjectFromTemplate": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetProjectConstruction": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetTimesheetEntry": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateTimesheetEntry": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "DestroyTimesheetEntry": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "RepositionTodolistGroup": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetTodolistOrGroup": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateTodolistOrGroup": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ListTodolistGroups": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateTodolistGroup": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ListTodos": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateTodo": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetEverythingCompletedTodos": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 5
      }
    },
    "GetEverythingNoDueDateTodos": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 5
      }
    },
    "GetEverythingOpenTodos": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 5
      }
    },
    "GetEverythingOverdueTodos": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetEverythingUnassignedTodos": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 5
      }
    },
    "GetTodo": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ReplaceTodo": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "CompleteTodo": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "UncompleteTodo": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "RepositionTodo": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "RepositionTodolist": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "GetTodoset": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetHillChart": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateHillChartSettings": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ListTodolists": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateTodolist": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetUpload": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateUpload": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ListUploadVersions": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateUploadVersion": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetVault": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateVault": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "ListDocuments": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateDocument": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ListUploads": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateUpload": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "ListVaults": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "pagination": {
        "style": "link",
        "totalCountHeader": "X-Total-Count",
        "maxPageSize": 50
      }
    },
    "CreateVault": {
      "retry": {
        "maxAttempts": 2,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "GetWebhook": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      }
    },
    "UpdateWebhook": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    },
    "DeleteWebhook": {
      "retry": {
        "maxAttempts": 3,
        "baseDelayMs": 1000,
        "backoff": "exponential",
        "retryOn": [
          429,
          503
        ]
      },
      "idempotent": {
        "natural": true
      }
    }
  }
};

export default metadata;

