/*
 * Copyright Basecamp, LLC
 * SPDX-License-Identifier: Apache-2.0
 */
package com.basecamp.smithy;

import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import software.amazon.smithy.model.node.ArrayNode;
import software.amazon.smithy.model.node.Node;
import software.amazon.smithy.model.node.ObjectNode;

import static org.junit.jupiter.api.Assertions.*;

class BareResponseExampleMapperTest {

    private BareResponseExampleMapper mapper;

    @BeforeEach
    void setUp() {
        mapper = new BareResponseExampleMapper();
    }

    /** Runs before the two schema mappers, which is what lets it read the wrapper key. */
    @Test
    void ordersAheadOfTheSchemaMappers() {
        assertTrue(mapper.getOrder() < new BareObjectResponseMapper().getOrder());
        assertTrue(mapper.getOrder() < new BareArrayResponseMapper().getOrder());
    }

    @Test
    void unwrapsAnObjectExampleToTheBareBody() {
        ObjectNode result = mapper.updateNode(null, null, document(
                objectResponseContent("GetProjectResponseContent", "project", "Project"),
                examples(ObjectNode.builder()
                        .withMember("example1", exampleWith(ObjectNode.builder()
                                .withMember("project", ObjectNode.builder()
                                        .withMember("id", 1)
                                        .withMember("name", "My Project")
                                        .build())
                                .build()))
                        .build(),
                        "GetProjectResponseContent")));

        ObjectNode value = exampleValue(result, "example1").expectObjectNode();
        assertEquals("My Project", value.expectStringMember("name").getValue());
        assertFalse(value.getMember("project").isPresent(), "wrapper key must be gone");
    }

    @Test
    void unwrapsAnArrayExampleToTheBareArray() {
        ObjectNode result = mapper.updateNode(null, null, document(
                arrayResponseContent("ListRecordingsResponseContent", "recordings", "Recording"),
                examples(ObjectNode.builder()
                        .withMember("example1", exampleWith(ObjectNode.builder()
                                .withMember("recordings", ArrayNode.fromNodes(
                                        ObjectNode.builder().withMember("id", 1).build()))
                                .build()))
                        .build(),
                        "ListRecordingsResponseContent")));

        Node value = exampleValue(result, "example1");
        assertTrue(value.isArrayNode(), "an array response's example must be an array, not an object");
        assertEquals(1, value.expectArrayNode().size());
    }

    /**
     * The #644 case: an `@examples` entry that declares only an `input` still
     * produces a response example, and the converter fills it with `{}`. That is
     * a documented 200 body no client could receive, so it is dropped rather
     * than published.
     */
    @Test
    void dropsAnExampleWithNoOutputInsteadOfPublishingAnEmptyObject() {
        ObjectNode result = mapper.updateNode(null, null, document(
                objectResponseContent("UpdateSubscriptionResponseContent", "subscription", "Subscription"),
                examples(ObjectNode.builder()
                        .withMember("hasOutput", exampleWith(ObjectNode.builder()
                                .withMember("subscription", ObjectNode.builder()
                                        .withMember("subscribed", true)
                                        .build())
                                .build()))
                        .withMember("inputOnly", exampleWith(ObjectNode.builder().build()))
                        .build(),
                        "UpdateSubscriptionResponseContent")));

        ObjectNode examples = mediaType(result).expectObjectMember("examples");
        assertTrue(examples.getMember("hasOutput").isPresent());
        assertFalse(examples.getMember("inputOnly").isPresent(), "an outputless example must be dropped");
    }

    /** An examples object emptied of every entry goes away entirely. */
    @Test
    void removesTheExamplesObjectWhenEveryEntryIsDropped() {
        ObjectNode result = mapper.updateNode(null, null, document(
                objectResponseContent("UpdateSubscriptionResponseContent", "subscription", "Subscription"),
                examples(ObjectNode.builder()
                        .withMember("inputOnly", exampleWith(ObjectNode.builder().build()))
                        .build(),
                        "UpdateSubscriptionResponseContent")));

        assertFalse(mediaType(result).getMember("examples").isPresent());
        assertTrue(mediaType(result).getMember("schema").isPresent(), "the rest of the media type survives");
    }

    /**
     * A response whose schema the sibling mappers leave alone — more than one
     * property, so nothing is unwrapped — keeps its example untouched.
     */
    @Test
    void leavesExamplesAloneWhenTheSchemaIsNotUnwrapped() {
        ObjectNode schema = ObjectNode.builder()
                .withMember("type", "object")
                .withMember("properties", ObjectNode.builder()
                        .withMember("person", ObjectNode.builder()
                                .withMember("$ref", "#/components/schemas/Person").build())
                        .withMember("events", ObjectNode.builder()
                                .withMember("type", "array").build())
                        .build())
                .build();

        ObjectNode original = ObjectNode.builder()
                .withMember("person", ObjectNode.builder().withMember("id", 1).build())
                .build();

        ObjectNode result = mapper.updateNode(null, null, document(
                ObjectNode.builder().withMember("GetPersonProgressResponseContent", schema).build(),
                examples(ObjectNode.builder()
                        .withMember("example1", exampleWith(original))
                        .build(),
                        "GetPersonProgressResponseContent")));

        assertEquals(original, exampleValue(result, "example1"));
    }

    @Test
    void toleratesADocumentWithNoPaths() {
        ObjectNode node = ObjectNode.builder()
                .withMember("components", ObjectNode.builder()
                        .withMember("schemas",
                                objectResponseContent("GetProjectResponseContent", "project", "Project"))
                        .build())
                .build();

        assertEquals(node, mapper.updateNode(null, null, node));
    }

    // --- fixture helpers -----------------------------------------------------

    private ObjectNode objectResponseContent(String name, String property, String target) {
        return ObjectNode.builder()
                .withMember(name, ObjectNode.builder()
                        .withMember("type", "object")
                        .withMember("properties", ObjectNode.builder()
                                .withMember(property, ObjectNode.builder()
                                        .withMember("$ref", "#/components/schemas/" + target)
                                        .build())
                                .build())
                        .build())
                .build();
    }

    private ObjectNode arrayResponseContent(String name, String property, String itemTarget) {
        return ObjectNode.builder()
                .withMember(name, ObjectNode.builder()
                        .withMember("type", "object")
                        .withMember("properties", ObjectNode.builder()
                                .withMember(property, ObjectNode.builder()
                                        .withMember("type", "array")
                                        .withMember("items", ObjectNode.builder()
                                                .withMember("$ref", "#/components/schemas/" + itemTarget)
                                                .build())
                                        .build())
                                .build())
                        .build())
                .build();
    }

    private ObjectNode exampleWith(ObjectNode value) {
        return ObjectNode.builder()
                .withMember("summary", "An example")
                .withMember("value", value)
                .build();
    }

    private ObjectNode examples(ObjectNode examples, String responseContentName) {
        return ObjectNode.builder()
                .withMember("/{accountId}/things.json", ObjectNode.builder()
                        .withMember("get", ObjectNode.builder()
                                .withMember("responses", ObjectNode.builder()
                                        .withMember("200", ObjectNode.builder()
                                                .withMember("content", ObjectNode.builder()
                                                        .withMember("application/json", ObjectNode.builder()
                                                                .withMember("schema", ObjectNode.builder()
                                                                        .withMember("$ref",
                                                                                "#/components/schemas/" + responseContentName)
                                                                        .build())
                                                                .withMember("examples", examples)
                                                                .build())
                                                        .build())
                                                .build())
                                        .build())
                                .build())
                        .build())
                .build();
    }

    private ObjectNode document(ObjectNode schemas, ObjectNode paths) {
        return ObjectNode.builder()
                .withMember("components", ObjectNode.builder().withMember("schemas", schemas).build())
                .withMember("paths", paths)
                .build();
    }

    private ObjectNode mediaType(ObjectNode document) {
        return document.expectObjectMember("paths")
                .expectObjectMember("/{accountId}/things.json")
                .expectObjectMember("get")
                .expectObjectMember("responses")
                .expectObjectMember("200")
                .expectObjectMember("content")
                .expectObjectMember("application/json");
    }

    private Node exampleValue(ObjectNode document, String exampleName) {
        return mediaType(document)
                .expectObjectMember("examples")
                .expectObjectMember(exampleName)
                .expectMember("value");
    }
}
