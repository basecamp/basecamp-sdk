/*
 * Copyright Basecamp, LLC
 * SPDX-License-Identifier: Apache-2.0
 */
package com.basecamp.smithy;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import software.amazon.smithy.model.node.ObjectNode;

import static org.junit.jupiter.api.Assertions.*;

class BareObjectResponseMapperTest {

    private BareObjectResponseMapper mapper;

    @BeforeEach
    void setUp() {
        mapper = new BareObjectResponseMapper();
    }

    @Test
    void shouldTransform_matchesGetResponseContentWithRef() {
        ObjectNode schema = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("project", ObjectNode.builder()
                                .withMember("$ref", "#/components/schemas/Project")
                                .build())
                        .build())
                .build();

        assertTrue(mapper.shouldTransform("GetProjectResponseContent", schema));
    }

    /**
     * An inline object property is NOT unwrapped: unwrapping it would lose the
     * property name, which for an inline shape is the only place the name
     * exists (e.g. attachable_sgid). Only a $ref is unwrapped, because the
     * referenced schema carries its own identity.
     */
    @Test
    void shouldTransform_rejectsInlineObjectProperty() {
        ObjectNode schema = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("thing", ObjectNode.builder()
                                .withMember("type", "object")
                                .build())
                        .build())
                .build();

        assertFalse(mapper.shouldTransform("GetThingResponseContent", schema));
    }

    /**
     * The operation-name prefix is irrelevant. BC3 returns a bare object from
     * GET, POST, PUT and action routes alike, so every single-$ref-property
     * *ResponseContent is unwrapped regardless of the verb it came from; a
     * List* response whose single property is an ARRAY is the sibling mapper's
     * job, not an exception to this one.
     */
    @Test
    void shouldTransform_ignoresTheOperationNamePrefix() {
        ObjectNode schema = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("project", ObjectNode.builder()
                                .withMember("$ref", "#/components/schemas/Project")
                                .build())
                        .build())
                .build();

        assertTrue(mapper.shouldTransform("ListProjectsResponseContent", schema));
        assertTrue(mapper.shouldTransform("UpdateProjectResponseContent", schema));
    }

    @Test
    void shouldTransform_rejectsNonResponseContentSuffix() {
        ObjectNode schema = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("project", ObjectNode.builder()
                                .withMember("$ref", "#/components/schemas/Project")
                                .build())
                        .build())
                .build();

        assertFalse(mapper.shouldTransform("GetProjectOutput", schema));
    }

    @Test
    void shouldTransform_rejectsMultipleProperties() {
        ObjectNode schema = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("person", ObjectNode.builder()
                                .withMember("$ref", "#/components/schemas/Person")
                                .build())
                        .withMember("todos", ObjectNode.builder()
                                .withMember("type", "array")
                                .build())
                        .withMember("grouped_by", ObjectNode.builder()
                                .withMember("type", "string")
                                .build())
                        .build())
                .build();

        assertFalse(mapper.shouldTransform("GetAssignedTodosResponseContent", schema));
    }

    @Test
    void shouldTransform_rejectsArrayProperty() {
        ObjectNode schema = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("events", ObjectNode.builder()
                                .withMember("type", "array")
                                .withMember("items", ObjectNode.builder()
                                        .withMember("$ref", "#/components/schemas/Event")
                                        .build())
                                .build())
                        .build())
                .build();

        assertFalse(mapper.shouldTransform("GetProjectTimelineResponseContent", schema));
    }

    @Test
    void shouldTransform_rejectsNonObjectType() {
        ObjectNode schema = ObjectNode.builder()
                .withMember("type", "array")
                .build();

        assertFalse(mapper.shouldTransform("GetProjectResponseContent", schema));
    }

    @Test
    void shouldTransform_rejectsNoProperties() {
        ObjectNode schema = ObjectNode.builder()
                .withMember("type", "object")
                .build();

        assertFalse(mapper.shouldTransform("GetProjectResponseContent", schema));
    }

    @Test
    void transformToRef_extractsRefSchema() {
        ObjectNode wrapped = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("project", ObjectNode.builder()
                                .withMember("$ref", "#/components/schemas/Project")
                                .build())
                        .build())
                .build();

        ObjectNode result = mapper.transformToRef(wrapped);

        assertEquals(
                "#/components/schemas/Project",
                result.expectStringMember("$ref").getValue()
        );
        // Should only have the $ref, no type or other keys
        assertFalse(result.getMember("type").isPresent());
    }

    @Test
    void transformToRef_extractsInlineObjectSchema() {
        ObjectNode wrapped = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("thing", ObjectNode.builder()
                                .withMember("type", "object")
                                .withMember("description", "An inline thing")
                                .build())
                        .build())
                .build();

        ObjectNode result = mapper.transformToRef(wrapped);

        assertEquals("object", result.expectStringMember("type").getValue());
        assertEquals("An inline thing", result.expectStringMember("description").getValue());
    }

    @Test
    void getOrder_returnsHighValue() {
        assertTrue(mapper.getOrder() > 0);
    }
}
