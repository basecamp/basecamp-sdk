package com.basecamp.sdk.generated.services

import com.basecamp.sdk.*
import com.basecamp.sdk.generated.models.*
import com.basecamp.sdk.services.BaseService
import kotlinx.serialization.json.JsonElement

/**
 * Service for People operations.
 *
 * @generated from OpenAPI spec — do not edit directly
 */
class PeopleService(client: AccountClient) : BaseService(client) {

    /**
     * List all account users who can be pinged
     * @param options Optional query parameters and pagination control
     */
    suspend fun listPingable(options: PaginationOptions? = null): ListResult<Person> {
        val info = OperationInfo(
            service = "People",
            operation = "ListPingablePeople",
            resourceType = "pingable_people",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return requestPaginated(info, options, {
            httpGet("/circles/people.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Person>>(body)
        }
    }

    /**
     * Get the current authenticated user's profile
     */
    suspend fun me(): Person {
        val info = OperationInfo(
            service = "People",
            operation = "GetMyProfile",
            resourceType = "my_profile",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/my/profile.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Person>(body)
        }
    }

    /**
     * List all people visible to the current user
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(options: PaginationOptions? = null): ListResult<Person> {
        val info = OperationInfo(
            service = "People",
            operation = "ListPeople",
            resourceType = "people",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return requestPaginated(info, options, {
            httpGet("/people.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Person>>(body)
        }
    }

    /**
     * Get a person by ID
     * @param personId The person ID
     */
    suspend fun get(personId: Long): Person {
        val info = OperationInfo(
            service = "People",
            operation = "GetPerson",
            resourceType = "person",
            isMutation = false,
            projectId = null,
            resourceId = personId,
        )
        return request(info, {
            httpGet("/people/${personId}", operationName = info.operation)
        }) { body ->
            json.decodeFromString<Person>(body)
        }
    }

    /**
     * List all active people on a project
     * @param projectId The project ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listForProject(projectId: Long, options: PaginationOptions? = null): ListResult<Person> {
        val info = OperationInfo(
            service = "People",
            operation = "ListProjectPeople",
            resourceType = "project_people",
            isMutation = false,
            projectId = projectId,
            resourceId = null,
        )
        return requestPaginated(info, options, {
            httpGet("/projects/${projectId}/people.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Person>>(body)
        }
    }

    /**
     * Update project access (grant/revoke/create people)
     * @param projectId The project ID
     * @param body Request body
     */
    suspend fun updateProjectAccess(projectId: Long, body: UpdateProjectAccessBody): JsonElement {
        val info = OperationInfo(
            service = "People",
            operation = "UpdateProjectAccess",
            resourceType = "project_access",
            isMutation = true,
            projectId = projectId,
            resourceId = null,
        )
        return request(info, {
            httpPut("/projects/${projectId}/people/users.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.grant?.let { put("grant", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
                body.revoke?.let { put("revoke", kotlinx.serialization.json.JsonArray(it.map { kotlinx.serialization.json.JsonPrimitive(it) })) }
                body.create?.let { put("create", kotlinx.serialization.json.JsonArray(it)) }
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * List people who can be assigned todos
     */
    suspend fun listAssignable(): List<Person> {
        val info = OperationInfo(
            service = "People",
            operation = "ListAssignablePeople",
            resourceType = "assignable_people",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/reports/todos/assigned.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Person>>(body)
        }
    }
}
