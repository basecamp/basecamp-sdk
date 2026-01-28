/*
 * Copyright Basecamp, LLC
 * SPDX-License-Identifier: Apache-2.0
 */
package com.basecamp.smithy;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import software.amazon.smithy.model.node.ObjectNode;

import static org.junit.jupiter.api.Assertions.*;

class BareArrayResponseMapperTest {

    private BareArrayResponseMapper mapper;

    @BeforeEach
    void setUp() {
        mapper = new BareArrayResponseMapper();
    }

    @Test
    void shouldTransform_matchesListResponseContent() {
        ObjectNode schema = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("projects", ObjectNode.builder()
                                .withMember("type", "array")
                                .withMember("items", ObjectNode.builder()
                                        .withMember("$ref", "#/components/schemas/Project")
                                        .build())
                                .build())
                        .build())
                .build();

        assertTrue(mapper.shouldTransform("ListProjectsResponseContent", schema));
    }

    @Test
    void shouldTransform_rejectsNonListPrefix() {
        ObjectNode schema = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("projects", ObjectNode.builder()
                                .withMember("type", "array")
                                .build())
                        .build())
                .build();

        assertFalse(mapper.shouldTransform("GetProjectsResponseContent", schema));
    }

    @Test
    void shouldTransform_rejectsNonResponseContentSuffix() {
        ObjectNode schema = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("projects", ObjectNode.builder()
                                .withMember("type", "array")
                                .build())
                        .build())
                .build();

        assertFalse(mapper.shouldTransform("ListProjectsOutput", schema));
    }

    @Test
    void shouldTransform_rejectsMultipleProperties() {
        ObjectNode schema = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("projects", ObjectNode.builder()
                                .withMember("type", "array")
                                .build())
                        .withMember("total", ObjectNode.builder()
                                .withMember("type", "integer")
                                .build())
                        .build())
                .build();

        assertFalse(mapper.shouldTransform("ListProjectsResponseContent", schema));
    }

    @Test
    void shouldTransform_rejectsNonArrayProperty() {
        ObjectNode schema = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("project", ObjectNode.builder()
                                .withMember("type", "object")
                                .build())
                        .build())
                .build();

        assertFalse(mapper.shouldTransform("ListProjectResponseContent", schema));
    }

    @Test
    void shouldTransform_rejectsNonObjectType() {
        ObjectNode schema = ObjectNode.builder()
                .withMember("type", "array")
                .build();

        assertFalse(mapper.shouldTransform("ListProjectsResponseContent", schema));
    }

    @Test
    void transformToArray_extractsArraySchema() {
        ObjectNode wrapped = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("projects", ObjectNode.builder()
                                .withMember("type", "array")
                                .withMember("items", ObjectNode.builder()
                                        .withMember("$ref", "#/components/schemas/Project")
                                        .build())
                                .build())
                        .build())
                .build();

        ObjectNode result = mapper.transformToArray(wrapped);

        assertEquals("array", result.expectStringMember("type").getValue());
        assertTrue(result.getObjectMember("items").isPresent());
        assertEquals(
                "#/components/schemas/Project",
                result.getObjectMember("items").get().expectStringMember("$ref").getValue()
        );
    }

    @Test
    void transformToArray_handlesArrayWithoutItems() {
        ObjectNode wrapped = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("things", ObjectNode.builder()
                                .withMember("type", "array")
                                .build())
                        .build())
                .build();

        ObjectNode result = mapper.transformToArray(wrapped);

        assertEquals("array", result.expectStringMember("type").getValue());
        assertFalse(result.getObjectMember("items").isPresent());
    }

    @Test
    void getOrder_returnsHighValue() {
        // Should run after core transformations (default order is 0)
        assertTrue(mapper.getOrder() > 0);
    }

    @Test
    void transformToArray_preservesArrayPropertyMetadata() {
        ObjectNode wrapped = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("projects", ObjectNode.builder()
                                .withMember("type", "array")
                                .withMember("description", "List of projects")
                                .withMember("title", "Projects")
                                .withMember("deprecated", true)
                                .withMember("items", ObjectNode.builder()
                                        .withMember("$ref", "#/components/schemas/Project")
                                        .build())
                                .build())
                        .build())
                .build();

        ObjectNode result = mapper.transformToArray(wrapped);

        assertEquals("array", result.expectStringMember("type").getValue());
        assertEquals("List of projects", result.expectStringMember("description").getValue());
        assertEquals("Projects", result.expectStringMember("title").getValue());
        assertTrue(result.expectBooleanMember("deprecated").getValue());
    }

    @Test
    void transformToArray_preservesVendorExtensions() {
        ObjectNode wrapped = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("x-wrapper-extension", "wrapper-value")
                .withMember("properties", ObjectNode.builder()
                        .withMember("items", ObjectNode.builder()
                                .withMember("type", "array")
                                .withMember("x-array-extension", "array-value")
                                .withMember("items", ObjectNode.builder()
                                        .withMember("type", "string")
                                        .build())
                                .build())
                        .build())
                .build();

        ObjectNode result = mapper.transformToArray(wrapped);

        // Array property extension takes precedence
        assertEquals("array-value", result.expectStringMember("x-array-extension").getValue());
        // Wrapper extension preserved if not overridden
        assertEquals("wrapper-value", result.expectStringMember("x-wrapper-extension").getValue());
    }

    @Test
    void transformToArray_arrayPropertyMetadataOverridesWrapper() {
        ObjectNode wrapped = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("description", "Wrapper description")
                .withMember("properties", ObjectNode.builder()
                        .withMember("data", ObjectNode.builder()
                                .withMember("type", "array")
                                .withMember("description", "Array description")
                                .withMember("items", ObjectNode.builder()
                                        .withMember("type", "string")
                                        .build())
                                .build())
                        .build())
                .build();

        ObjectNode result = mapper.transformToArray(wrapped);

        // Array property description should win
        assertEquals("Array description", result.expectStringMember("description").getValue());
    }

    @Test
    void transformToArray_inheritsWrapperMetadataWhenArrayHasNone() {
        ObjectNode wrapped = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("description", "Wrapper description")
                .withMember("title", "Wrapper Title")
                .withMember("properties", ObjectNode.builder()
                        .withMember("data", ObjectNode.builder()
                                .withMember("type", "array")
                                .withMember("items", ObjectNode.builder()
                                        .withMember("type", "string")
                                        .build())
                                .build())
                        .build())
                .build();

        ObjectNode result = mapper.transformToArray(wrapped);

        // Should inherit from wrapper since array has no metadata
        assertEquals("Wrapper description", result.expectStringMember("description").getValue());
        assertEquals("Wrapper Title", result.expectStringMember("title").getValue());
    }

    @Test
    void transformToArray_inheritsWrapperNullableAndDeprecated() {
        ObjectNode wrapped = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("nullable", true)
                .withMember("deprecated", true)
                .withMember("properties", ObjectNode.builder()
                        .withMember("data", ObjectNode.builder()
                                .withMember("type", "array")
                                .withMember("items", ObjectNode.builder()
                                        .withMember("type", "string")
                                        .build())
                                .build())
                        .build())
                .build();

        ObjectNode result = mapper.transformToArray(wrapped);

        // Should inherit nullable and deprecated from wrapper
        assertTrue(result.expectBooleanMember("nullable").getValue());
        assertTrue(result.expectBooleanMember("deprecated").getValue());
    }

    @Test
    void transformToArray_preservesArrayKeywords() {
        ObjectNode wrapped = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("data", ObjectNode.builder()
                                .withMember("type", "array")
                                .withMember("minItems", 1)
                                .withMember("maxItems", 100)
                                .withMember("uniqueItems", true)
                                .withMember("readOnly", true)
                                .withMember("items", ObjectNode.builder()
                                        .withMember("type", "string")
                                        .build())
                                .build())
                        .build())
                .build();

        ObjectNode result = mapper.transformToArray(wrapped);

        assertEquals(1, result.expectNumberMember("minItems").getValue().intValue());
        assertEquals(100, result.expectNumberMember("maxItems").getValue().intValue());
        assertTrue(result.expectBooleanMember("uniqueItems").getValue());
        assertTrue(result.expectBooleanMember("readOnly").getValue());
    }
}
