/*
 * Copyright Basecamp, LLC
 * SPDX-License-Identifier: Apache-2.0
 *
 * Transforms List*ResponseContent schemas from wrapped objects to bare arrays.
 * This bridges the gap between Smithy's protocol constraints (which require
 * wrapped structures) and the BC3 API's actual wire format (bare arrays).
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
 * An OpenAPI mapper that transforms List response schemas from wrapped objects
 * to bare arrays, matching the BC3 API's actual response format.
 *
 * <p>Smithy's AWS restJson1 protocol requires list outputs to be modeled as
 * wrapped structures (e.g., {@code ListProjectsOutput { projects: ProjectList }})
 * because {@code @httpPayload} only supports structures, not arrays.
 *
 * <p>However, the BC3 API returns bare arrays: {@code GET /projects.json}
 * returns {@code [...]} not {@code {"projects": [...]}}.
 *
 * <p>This mapper runs after core OpenAPI generation and transforms schemas
 * matching the pattern {@code List*ResponseContent} from:
 * <pre>{@code
 * {"type": "object", "properties": {"x": {"type": "array", "items": ...}}}
 * }</pre>
 * to:
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
            LOGGER.info("Transformed " + transformedCount + " List*ResponseContent schemas to bare arrays");
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
        // Must match List*ResponseContent pattern
        if (!name.startsWith("List") || !name.endsWith("ResponseContent")) {
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

    /**
     * Transforms a wrapped object schema to a bare array schema.
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

        return result.build();
    }
}
