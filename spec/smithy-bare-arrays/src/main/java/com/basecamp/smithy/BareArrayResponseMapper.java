/*
 * Copyright Basecamp, LLC
 * SPDX-License-Identifier: Apache-2.0
 *
 * Transforms *ResponseContent schemas from wrapped objects to bare arrays.
 * This bridges the gap between Smithy's protocol constraints (which require
 * wrapped structures) and the BC3 API's actual wire format (bare arrays).
 *
 * Applies to any response schema ending in ResponseContent that has exactly
 * one property which is an array type. This includes List operations, Search,
 * and Get operations that return collections (e.g., timesheets).
 */
package com.basecamp.smithy;

import software.amazon.smithy.model.node.Node;
import software.amazon.smithy.model.node.ObjectNode;
import software.amazon.smithy.model.traits.Trait;
import software.amazon.smithy.openapi.fromsmithy.Context;
import software.amazon.smithy.openapi.fromsmithy.OpenApiMapper;
import software.amazon.smithy.openapi.model.OpenApi;

import java.util.Map;
import java.util.logging.Logger;

/**
 * An OpenAPI mapper that transforms response schemas from wrapped objects
 * to bare arrays, matching the BC3 API's actual response format.
 *
 * <p>Smithy's AWS restJson1 protocol requires list outputs to be modeled as
 * wrapped structures (e.g., {@code ListProjectsOutput { projects: ProjectList }})
 * because {@code @httpPayload} only supports structures, not arrays.
 *
 * <p>However, the BC3 API returns bare arrays for all array-returning operations:
 * {@code GET /projects.json} returns {@code [...]} not {@code {"projects": [...]}}.
 * This applies to List operations, Search, and Get operations that return
 * collections (e.g., timesheets).
 *
 * <p>This mapper runs after core OpenAPI generation and transforms ALL
 * {@code *ResponseContent} schemas that have exactly one property which is
 * an array type. For example:
 * <pre>{@code
 * {"type": "object", "properties": {"x": {"type": "array", "items": ...}}}
 * }</pre>
 * becomes:
 * <pre>{@code
 * {"type": "array", "items": ...}
 * }</pre>
 */
public final class BareArrayResponseMapper implements OpenApiMapper {

    private static final Logger LOGGER = Logger.getLogger(BareArrayResponseMapper.class.getName());

    @Override
    public byte getOrder() {
        // Run after core transformations (default order is 0)
        return 100;
    }

    @Override
    public ObjectNode updateNode(Context<? extends Trait> context, OpenApi openapi, ObjectNode node) {
        ObjectNode componentsNode = node.getObjectMember("components").orElse(null);
        if (componentsNode == null) {
            return node;
        }

        ObjectNode schemasNode = componentsNode.getObjectMember("schemas").orElse(null);
        if (schemasNode == null) {
            return node;
        }

        ObjectNode.Builder newSchemas = ObjectNode.builder();
        int transformedCount = 0;

        for (Map.Entry<String, Node> entry : schemasNode.getStringMap().entrySet()) {
            String name = entry.getKey();
            Node schema = entry.getValue();

            if (shouldTransform(name, schema)) {
                newSchemas.withMember(name, transformToArray(schema.expectObjectNode()));
                transformedCount++;
            } else {
                newSchemas.withMember(name, schema);
            }
        }

        if (transformedCount > 0) {
            LOGGER.info("Transformed " + transformedCount + " *ResponseContent schemas to bare arrays");
        }

        // Rebuild the node with updated schemas
        ObjectNode newComponents = componentsNode.toBuilder()
                .withMember("schemas", newSchemas.build())
                .build();

        return node.toBuilder()
                .withMember("components", newComponents)
                .build();
    }

    /**
     * Determines if a schema should be transformed.
     *
     * @param name   the schema name
     * @param schema the schema node
     * @return true if the schema matches the criteria for transformation
     */
    boolean shouldTransform(String name, Node schema) {
        // Must be a *ResponseContent schema
        if (!name.endsWith("ResponseContent")) {
            return false;
        }

        if (!schema.isObjectNode()) {
            return false;
        }

        ObjectNode obj = schema.expectObjectNode();

        // Must be type: "object"
        if (!obj.getStringMember("type").map(n -> n.getValue().equals("object")).orElse(false)) {
            return false;
        }

        // Must have exactly one property that is an array
        ObjectNode properties = obj.getObjectMember("properties").orElse(null);
        if (properties == null) {
            return false;
        }

        Map<String, Node> props = properties.getStringMap();
        if (props.size() != 1) {
            return false;
        }

        // The single property must be an array type
        Node propValue = props.values().iterator().next();
        if (!propValue.isObjectNode()) {
            return false;
        }

        return propValue.expectObjectNode()
                .getStringMember("type")
                .map(n -> n.getValue().equals("array"))
                .orElse(false);
    }

        // Structural members that should not be copied from source schemas
    private static final java.util.Set<String> STRUCTURAL_MEMBERS = java.util.Set.of(
            "type", "properties", "required", "items", "additionalProperties"
    );

    /**
     * Transforms a wrapped object schema to a bare array schema.
     * Preserves all non-structural metadata from both the array property
     * (higher priority) and the wrapper object (fallback).
     *
     * @param wrapped the wrapped object schema
     * @return the bare array schema
     */
    ObjectNode transformToArray(ObjectNode wrapped) {
        ObjectNode properties = wrapped.getObjectMember("properties").get();
        ObjectNode arrayProp = properties.getStringMap().values().iterator().next().expectObjectNode();

        ObjectNode.Builder result = ObjectNode.builder()
                .withMember("type", "array");

        // Preserve the items definition
        arrayProp.getObjectMember("items").ifPresent(items ->
                result.withMember("items", items));

        // Preserve all non-structural metadata from array property (takes precedence)
        copyNonStructuralMembers(arrayProp, result);

        // Preserve metadata from wrapper if not already set by array property
        copyNonStructuralMembersIfAbsent(wrapped, result, arrayProp);

        return result.build();
    }

    /**
     * Copies all non-structural members from source to builder.
     * This includes description, title, nullable, deprecated, minItems, maxItems,
     * uniqueItems, readOnly, writeOnly, default, example, examples, and vendor extensions.
     */
    private void copyNonStructuralMembers(ObjectNode source, ObjectNode.Builder builder) {
        for (Map.Entry<String, Node> entry : source.getStringMap().entrySet()) {
            String key = entry.getKey();
            if (!STRUCTURAL_MEMBERS.contains(key)) {
                builder.withMember(key, entry.getValue());
            }
        }
    }

    /**
     * Copies non-structural members from source only if not already present in higherPriority.
     */
    private void copyNonStructuralMembersIfAbsent(ObjectNode source, ObjectNode.Builder builder, ObjectNode higherPriority) {
        for (Map.Entry<String, Node> entry : source.getStringMap().entrySet()) {
            String key = entry.getKey();
            if (!STRUCTURAL_MEMBERS.contains(key) && !higherPriority.getMember(key).isPresent()) {
                builder.withMember(key, entry.getValue());
            }
        }
    }
}
