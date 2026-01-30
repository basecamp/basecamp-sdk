/*
 * Copyright Basecamp, LLC
 * SPDX-License-Identifier: Apache-2.0
 *
 * Transforms Get*ResponseContent schemas from wrapped objects to bare $ref.
 * This bridges the gap between Smithy's protocol constraints (which require
 * wrapped structures) and the BC3 API's actual wire format (bare objects).
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
 * An OpenAPI mapper that transforms Get response schemas from wrapped objects
 * to bare {@code $ref}, matching the BC3 API's actual response format.
 *
 * <p>Smithy's AWS restJson1 protocol requires outputs to be modeled as
 * wrapped structures (e.g., {@code GetProjectOutput { project: Project }})
 * because {@code @httpPayload} only supports structures, not bare references.
 *
 * <p>However, the BC3 API returns bare objects: {@code GET /projects/123.json}
 * returns a project object directly, not {@code {"project": {...}}}.
 *
 * <p>This mapper runs after core OpenAPI generation and transforms schemas
 * matching the pattern {@code Get*ResponseContent} from:
 * <pre>{@code
 * {"type": "object", "properties": {"project": {"$ref": "#/components/schemas/Project"}}}
 * }</pre>
 * to:
 * <pre>{@code
 * {"$ref": "#/components/schemas/Project"}
 * }</pre>
 *
 * <p>Only schemas with exactly one property that is NOT an array are transformed.
 * Multi-field responses (e.g., GetAssignedTodosResponseContent) are left as-is.
 */
public final class BareObjectResponseMapper implements OpenApiMapper {

    private static final Logger LOGGER = Logger.getLogger(BareObjectResponseMapper.class.getName());

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
                newSchemas.withMember(name, transformToRef(schema.expectObjectNode()));
                transformedCount++;
            } else {
                newSchemas.withMember(name, schema);
            }
        }

        if (transformedCount > 0) {
            LOGGER.info("Transformed " + transformedCount + " Get*ResponseContent schemas to bare $ref");
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
        // Must match Get*ResponseContent pattern
        if (!name.startsWith("Get") || !name.endsWith("ResponseContent")) {
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

        // Must have exactly one property
        ObjectNode properties = obj.getObjectMember("properties").orElse(null);
        if (properties == null) {
            return false;
        }

        Map<String, Node> props = properties.getStringMap();
        if (props.size() != 1) {
            return false;
        }

        // The single property must NOT be an array (i.e., it's a $ref or inline object)
        Node propValue = props.values().iterator().next();
        if (!propValue.isObjectNode()) {
            return false;
        }

        ObjectNode propObj = propValue.expectObjectNode();

        // If it has a $ref, it's a reference — transform it
        if (propObj.getMember("$ref").isPresent()) {
            return true;
        }

        // If it's type: "array", skip (handled by BareArrayResponseMapper)
        boolean isArray = propObj.getStringMember("type")
                .map(n -> n.getValue().equals("array"))
                .orElse(false);

        return !isArray;
    }

    /**
     * Transforms a wrapped object schema to a bare $ref or inline schema.
     * Extracts the single property's value as the replacement schema.
     *
     * @param wrapped the wrapped object schema
     * @return the bare schema (typically a $ref node)
     */
    ObjectNode transformToRef(ObjectNode wrapped) {
        ObjectNode properties = wrapped.getObjectMember("properties").get();
        return properties.getStringMap().values().iterator().next().expectObjectNode();
    }
}
