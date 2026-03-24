/*
 * Copyright Basecamp, LLC
 * SPDX-License-Identifier: Apache-2.0
 *
 * Transforms operations annotated with x-basecamp-multipart from
 * application/octet-stream to multipart/form-data in the OpenAPI output.
 *
 * The Smithy model uses @httpPayload Blob which produces octet-stream.
 * This mapper detects the x-basecamp-multipart extension (bridged from
 * the @basecampMultipart trait) and rewrites the request body to
 * multipart/form-data with the specified field name.
 */
package com.basecamp.smithy;

import software.amazon.smithy.model.node.Node;
import software.amazon.smithy.model.node.ObjectNode;
import software.amazon.smithy.model.node.StringNode;
import software.amazon.smithy.model.traits.Trait;
import software.amazon.smithy.openapi.fromsmithy.Context;
import software.amazon.smithy.openapi.fromsmithy.OpenApiMapper;
import software.amazon.smithy.openapi.model.OpenApi;

import java.util.Map;
import java.util.logging.Logger;

/**
 * Rewrites request bodies from octet-stream to multipart/form-data for
 * operations marked with {@code x-basecamp-multipart}.
 */
public final class MultipartRequestBodyMapper implements OpenApiMapper {

    private static final Logger LOGGER = Logger.getLogger(MultipartRequestBodyMapper.class.getName());
    private static final String EXTENSION_KEY = "x-basecamp-multipart";

    @Override
    public byte getOrder() {
        // Run after core transformations but before other custom mappers
        return 90;
    }

    @Override
    public ObjectNode updateNode(Context<? extends Trait> context, OpenApi openapi, ObjectNode node) {
        ObjectNode pathsNode = node.getObjectMember("paths").orElse(null);
        if (pathsNode == null) {
            return node;
        }

        ObjectNode.Builder newPaths = ObjectNode.builder();
        int transformedCount = 0;

        for (Map.Entry<String, Node> pathEntry : pathsNode.getStringMap().entrySet()) {
            String path = pathEntry.getKey();
            ObjectNode pathItem = pathEntry.getValue().expectObjectNode();

            ObjectNode.Builder newPathItem = ObjectNode.builder();
            boolean pathChanged = false;

            for (Map.Entry<String, Node> methodEntry : pathItem.getStringMap().entrySet()) {
                String method = methodEntry.getKey();
                Node operationNode = methodEntry.getValue();

                if (!operationNode.isObjectNode()) {
                    newPathItem.withMember(method, operationNode);
                    continue;
                }

                ObjectNode operation = operationNode.expectObjectNode();
                ObjectNode multipartExt = operation.getObjectMember(EXTENSION_KEY).orElse(null);

                if (multipartExt != null) {
                    String fieldName = multipartExt.getStringMember("field")
                            .map(StringNode::getValue)
                            .orElse("file");

                    ObjectNode transformed = transformToMultipart(operation, fieldName);
                    newPathItem.withMember(method, transformed);
                    pathChanged = true;
                    transformedCount++;
                    LOGGER.info("Transformed " + method.toUpperCase() + " " + path + " to multipart/form-data (field: " + fieldName + ")");
                } else {
                    newPathItem.withMember(method, operation);
                }
            }

            newPaths.withMember(path, pathChanged ? newPathItem.build() : pathItem);
        }

        if (transformedCount == 0) {
            return node;
        }

        return node.toBuilder()
                .withMember("paths", newPaths.build())
                .build();
    }

    /**
     * Rewrites an operation's requestBody from octet-stream to multipart/form-data.
     */
    private ObjectNode transformToMultipart(ObjectNode operation, String fieldName) {
        ObjectNode requestBody = operation.getObjectMember("requestBody").orElse(null);
        if (requestBody == null) {
            return operation;
        }

        ObjectNode content = requestBody.getObjectMember("content").orElse(null);
        if (content == null) {
            return operation;
        }

        // Build the multipart/form-data schema: { type: object, properties: { <field>: { type: string, format: binary } }, required: [<field>] }
        ObjectNode fileSchema = ObjectNode.builder()
                .withMember("type", "string")
                .withMember("format", "binary")
                .build();

        ObjectNode multipartSchema = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember(fieldName, fileSchema)
                        .build())
                .withMember("required", Node.fromStrings(fieldName))
                .build();

        ObjectNode newContent = ObjectNode.builder()
                .withMember("multipart/form-data", ObjectNode.builder()
                        .withMember("schema", multipartSchema)
                        .build())
                .build();

        // Preserve all original requestBody members (description, extensions, etc.)
        // except content, which we're replacing
        ObjectNode.Builder newRequestBody = ObjectNode.builder();
        for (Map.Entry<String, Node> entry : requestBody.getStringMap().entrySet()) {
            if (!"content".equals(entry.getKey())) {
                newRequestBody.withMember(entry.getKey(), entry.getValue());
            }
        }
        newRequestBody.withMember("content", newContent);

        return operation.toBuilder()
                .withMember("requestBody", newRequestBody.build())
                .build();
    }
}
