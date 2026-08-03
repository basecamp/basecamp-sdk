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
     * Get the current user's preferences
     */
    suspend fun myPreferences(): JsonElement {
        val info = OperationInfo(
            service = "People",
            operation = "GetMyPreferences",
            resourceType = "my_preference",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpGet("/my/preferences.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Update the current user's preferences.
     * @param body Request body
     */
    suspend fun updateMyPreferences(body: UpdateMyPreferencesBody): JsonElement {
        val info = OperationInfo(
            service = "People",
            operation = "UpdateMyPreferences",
            resourceType = "my_preference",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        return request(info, {
            httpPut("/my/preferences.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("person", body.person)
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
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
     * Update the current authenticated user's profile
     * @param body Request body
     */
    suspend fun updateMyProfile(body: UpdateMyProfileBody): Unit {
        val info = OperationInfo(
            service = "People",
            operation = "UpdateMyProfile",
            resourceType = "my_profile",
            isMutation = true,
            projectId = null,
            resourceId = null,
        )
        request(info, {
            httpPut("/my/profile.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                body.name?.let { put("name", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.emailAddress?.let { put("email_address", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.title?.let { put("title", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.bio?.let { put("bio", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.location?.let { put("location", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.timeZoneName?.let { put("time_zone_name", kotlinx.serialization.json.JsonPrimitive(it)) }
                body.firstWeekDay?.let { put("first_week_day", it) }
                body.timeFormat?.let { put("time_format", kotlinx.serialization.json.JsonPrimitive(it)) }
            }), operationName = info.operation)
        }) { Unit }
    }

    /**
     * List all people visible to the current user
     * @param options Optional query parameters and pagination control
     */
    suspend fun list(options: ListPeopleOptions): ListResult<Person> {
        val info = OperationInfo(
            service = "People",
            operation = "ListPeople",
            resourceType = "people",
            isMutation = false,
            projectId = null,
            resourceId = null,
        )
        val qs = buildQueryString(
            "page" to options.page,
        )
        return requestPaginated(info, options.toPaginationOptions(), {
            httpGet("/people.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Person>>(body)
        }
    }

    /**
     * Source-compatibility overload: the signature this operation had before
     * it gained query parameters of its own.
     *
     * Prefer [ListPeopleOptions], which also carries this operation's query
     * parameters. This overload forwards maxItems and leaves them unset.
     *
     * Because two candidates now apply, an *untyped* callable reference to
     * [list] needs an expected type to disambiguate.
     */
    suspend fun list(options: PaginationOptions? = null): ListResult<Person> =
        list(ListPeopleOptions(maxItems = options?.maxItems, page = options?.page))

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
     * Get the out of office status for a person
     * @param personId The person ID
     */
    suspend fun outOfOffice(personId: Long): JsonElement {
        val info = OperationInfo(
            service = "People",
            operation = "GetOutOfOffice",
            resourceType = "out_of_office",
            isMutation = false,
            projectId = null,
            resourceId = personId,
        )
        return request(info, {
            httpGet("/people/${personId}/out_of_office.json", operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Enable or replace out of office for a person.
     * @param personId The person ID
     * @param body Request body
     */
    suspend fun enableOutOfOffice(personId: Long, body: EnableOutOfOfficeBody): JsonElement {
        val info = OperationInfo(
            service = "People",
            operation = "EnableOutOfOffice",
            resourceType = "out_of_office",
            isMutation = true,
            projectId = null,
            resourceId = personId,
        )
        return request(info, {
            httpPost("/people/${personId}/out_of_office.json", json.encodeToString(kotlinx.serialization.json.buildJsonObject {
                put("out_of_office", body.outOfOffice)
            }), operationName = info.operation)
        }) { body ->
            json.decodeFromString<JsonElement>(body)
        }
    }

    /**
     * Disable out of office for a person.
     * @param personId The person ID
     */
    suspend fun disableOutOfOffice(personId: Long): Unit {
        val info = OperationInfo(
            service = "People",
            operation = "DisableOutOfOffice",
            resourceType = "out_of_office",
            isMutation = true,
            projectId = null,
            resourceId = personId,
        )
        request(info, {
            httpDelete("/people/${personId}/out_of_office.json", operationName = info.operation)
        }) { Unit }
    }

    /**
     * List all active people on a project
     * @param projectId The project ID
     * @param options Optional query parameters and pagination control
     */
    suspend fun listForProject(projectId: Long, options: ListProjectPeopleOptions): ListResult<Person> {
        val info = OperationInfo(
            service = "People",
            operation = "ListProjectPeople",
            resourceType = "project_people",
            isMutation = false,
            projectId = projectId,
            resourceId = null,
        )
        val qs = buildQueryString(
            "page" to options.page,
        )
        return requestPaginated(info, options.toPaginationOptions(), {
            httpGet("/projects/${projectId}/people.json" + qs, operationName = info.operation)
        }) { body ->
            json.decodeFromString<List<Person>>(body)
        }
    }

    /**
     * Source-compatibility overload: the signature this operation had before
     * it gained query parameters of its own.
     *
     * Prefer [ListProjectPeopleOptions], which also carries this operation's query
     * parameters. This overload forwards maxItems and leaves them unset.
     *
     * Because two candidates now apply, an *untyped* callable reference to
     * [listForProject] needs an expected type to disambiguate.
     */
    suspend fun listForProject(projectId: Long, options: PaginationOptions? = null): ListResult<Person> =
        listForProject(projectId, ListProjectPeopleOptions(maxItems = options?.maxItems, page = options?.page))

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
