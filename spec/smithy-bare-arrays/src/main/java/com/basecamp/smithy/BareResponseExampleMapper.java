/*
 * Copyright Basecamp, LLC
 * SPDX-License-Identifier: Apache-2.0
 *
 * Unwraps response EXAMPLES to match the response SCHEMAS that
 * BareObjectResponseMapper and BareArrayResponseMapper unwrap.
 */
package com.basecamp.smithy;

import software.amazon.smithy.model.node.Node;
import software.amazon.smithy.model.node.ObjectNode;
import software.amazon.smithy.model.traits.Trait;
import software.amazon.smithy.openapi.fromsmithy.Context;
import software.amazon.smithy.openapi.fromsmithy.OpenApiMapper;
import software.amazon.smithy.openapi.model.OpenApi;

import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.logging.Logger;

/**
 * Unwraps the response examples belonging to schemas the two bare-response
 * mappers rewrite, and drops the ones that have nothing to unwrap.
 *
 * <p>Smithy's restJson1 protocol forces an operation output to be a wrapper
 * structure ({@code GetProjectOutput { project: Project }}), and the sibling
 * mappers rewrite the projected {@code *ResponseContent} schema down to the bare
 * shape BC3 actually sends. They rewrite only {@code components.schemas}, so
 * until this mapper existed the projected {@code examples} kept the wrapper and
 * contradicted the very schema they sit beside — {@code {"result": {…}}} against
 * a schema that is a bare {@code Todolist}.
 *
 * <p>The second half matters more. The OpenAPI converter emits a response
 * example for every {@code @examples} entry, <em>including entries that declare
 * only an {@code input}</em>, and fills those with the empty object. Eight of
 * the ten response examples on {@code main} were {@code {}} — documented 200
 * bodies no client could receive, one of them {@code {}} where the schema is an
 * array (basecamp/basecamp-sdk#644). An example with no output to show is
 * removed here rather than published as an empty object, so declaring
 * {@code input}-only examples stays a legitimate thing to do.
 *
 * <p>Runs at order 90, ahead of both sibling mappers, because it needs to read
 * the wrapper property name off the still-wrapped schema. It reuses their own
 * {@code shouldTransform} predicates rather than restating them, so the three
 * cannot drift apart on which schemas count as wrapped.
 */
public final class BareResponseExampleMapper implements OpenApiMapper {

    private static final Logger LOGGER = Logger.getLogger(BareResponseExampleMapper.class.getName());

    private final BareObjectResponseMapper objectMapper = new BareObjectResponseMapper();
    private final BareArrayResponseMapper arrayMapper = new BareArrayResponseMapper();

    @Override
    public byte getOrder() {
        // Ahead of the two mappers whose schema rewrites this mirrors (both 100):
        // the wrapper property name is only readable before they run.
        return 90;
    }

    @Override
    public ObjectNode updateNode(Context<? extends Trait> context, OpenApi openapi, ObjectNode node) {
        Map<String, String> wrapperKeys = collectWrapperKeys(node);
        if (wrapperKeys.isEmpty()) {
            return node;
        }

        ObjectNode pathsNode = node.getObjectMember("paths").orElse(null);
        if (pathsNode == null) {
            return node;
        }

        Counters counters = new Counters();
        ObjectNode newPaths = mapMembers(pathsNode, (path, pathItem) ->
                pathItem.isObjectNode()
                        ? mapMembers(pathItem.expectObjectNode(), (method, operation) ->
                                operation.isObjectNode()
                                        ? rewriteOperation(operation.expectObjectNode(), wrapperKeys, counters)
                                        : operation)
                        : pathItem);

        if (counters.unwrapped > 0 || counters.dropped > 0) {
            LOGGER.info("Unwrapped " + counters.unwrapped + " response examples, dropped "
                    + counters.dropped + " with no output to show");
        }

        return node.toBuilder().withMember("paths", newPaths).build();
    }

    /**
     * Maps every {@code *ResponseContent} schema the sibling mappers will unwrap
     * to the name of the single wrapper property they will unwrap it to.
     */
    private Map<String, String> collectWrapperKeys(ObjectNode node) {
        Map<String, String> wrapperKeys = new HashMap<>();

        ObjectNode schemas = node.getObjectMember("components")
                .flatMap(components -> components.getObjectMember("schemas"))
                .orElse(null);
        if (schemas == null) {
            return wrapperKeys;
        }

        for (Map.Entry<String, Node> entry : schemas.getStringMap().entrySet()) {
            String name = entry.getKey();
            Node schema = entry.getValue();
            if (!objectMapper.shouldTransform(name, schema) && !arrayMapper.shouldTransform(name, schema)) {
                continue;
            }
            schema.expectObjectNode()
                    .getObjectMember("properties")
                    .map(properties -> properties.getStringMap().keySet().iterator().next())
                    .ifPresent(property -> wrapperKeys.put(name, property));
        }

        return wrapperKeys;
    }

    private Node rewriteOperation(ObjectNode operation, Map<String, String> wrapperKeys, Counters counters) {
        ObjectNode responses = operation.getObjectMember("responses").orElse(null);
        if (responses == null) {
            return operation;
        }

        ObjectNode newResponses = mapMembers(responses, (status, response) ->
                response.isObjectNode()
                        ? rewriteResponse(response.expectObjectNode(), wrapperKeys, counters)
                        : response);

        return operation.toBuilder().withMember("responses", newResponses).build();
    }

    private Node rewriteResponse(ObjectNode response, Map<String, String> wrapperKeys, Counters counters) {
        ObjectNode content = response.getObjectMember("content").orElse(null);
        if (content == null) {
            return response;
        }

        ObjectNode newContent = mapMembers(content, (mediaType, mediaTypeObject) ->
                mediaTypeObject.isObjectNode()
                        ? rewriteMediaType(mediaTypeObject.expectObjectNode(), wrapperKeys, counters)
                        : mediaTypeObject);

        return response.toBuilder().withMember("content", newContent).build();
    }

    private Node rewriteMediaType(ObjectNode mediaTypeObject, Map<String, String> wrapperKeys, Counters counters) {
        ObjectNode examples = mediaTypeObject.getObjectMember("examples").orElse(null);
        if (examples == null) {
            return mediaTypeObject;
        }

        String wrapperKey = mediaTypeObject.getObjectMember("schema")
                .flatMap(schema -> schema.getStringMember("$ref"))
                .map(ref -> ref.getValue().substring(ref.getValue().lastIndexOf('/') + 1))
                .map(wrapperKeys::get)
                .orElse(null);
        if (wrapperKey == null) {
            return mediaTypeObject;
        }

        ObjectNode.Builder kept = ObjectNode.builder();
        int keptCount = 0;
        for (Map.Entry<String, Node> entry : examples.getStringMap().entrySet()) {
            Node unwrapped = unwrapExample(entry.getValue(), wrapperKey);
            if (unwrapped == null) {
                counters.dropped++;
                continue;
            }
            kept.withMember(entry.getKey(), unwrapped);
            keptCount++;
            counters.unwrapped++;
        }

        ObjectNode.Builder result = ObjectNode.builder();
        for (Map.Entry<String, Node> entry : mediaTypeObject.getStringMap().entrySet()) {
            if (entry.getKey().equals("examples")) {
                // An examples object emptied of every entry is dropped outright:
                // OpenAPI has no use for `"examples": {}` and leaving it behind
                // would just be a different empty thing to document.
                if (keptCount > 0) {
                    result.withMember("examples", kept.build());
                }
            } else {
                result.withMember(entry.getKey(), entry.getValue());
            }
        }
        return result.build();
    }

    /**
     * Rewrites one example object, or returns null when it carries no output.
     *
     * <p>The value is the Smithy {@code output} node verbatim, so it is the
     * wrapper structure: extracting the single member yields the bare body. A
     * value that is not an object, or that lacks the wrapper key, is an example
     * whose {@code @examples} entry declared no {@code output} — there is
     * nothing to show, and the converter's stand-in for it is the empty object.
     */
    private Node unwrapExample(Node example, String wrapperKey) {
        if (!example.isObjectNode()) {
            return null;
        }

        ObjectNode exampleObject = example.expectObjectNode();
        Node value = exampleObject.getMember("value").orElse(null);
        if (value == null || !value.isObjectNode()) {
            return null;
        }

        Node unwrapped = value.expectObjectNode().getMember(wrapperKey).orElse(null);
        if (unwrapped == null) {
            return null;
        }

        return exampleObject.toBuilder().withMember("value", unwrapped).build();
    }

    /** Rebuilds an object node, replacing each member value through the mapper. */
    private ObjectNode mapMembers(ObjectNode source, MemberMapper mapper) {
        Map<String, Node> members = new LinkedHashMap<>(source.getStringMap());
        ObjectNode.Builder builder = ObjectNode.builder();
        for (Map.Entry<String, Node> entry : members.entrySet()) {
            builder.withMember(entry.getKey(), mapper.apply(entry.getKey(), entry.getValue()));
        }
        return builder.build();
    }

    @FunctionalInterface
    private interface MemberMapper {
        Node apply(String key, Node value);
    }

    private static final class Counters {
        private int unwrapped;
        private int dropped;
    }
}
