/*
 * Copyright Basecamp, LLC
 * SPDX-License-Identifier: Apache-2.0
 */
package com.basecamp.smithy;

import software.amazon.smithy.openapi.fromsmithy.OpenApiMapper;
import software.amazon.smithy.openapi.fromsmithy.Smithy2OpenApiExtension;

import java.util.List;

/**
 * Smithy extension that registers the BareArrayResponseMapper.
 *
 * <p>This class is discovered via Java SPI and automatically registers
 * the mapper when Smithy builds OpenAPI specifications.
 *
 * <p>BareResponseExampleMapper is listed alongside the two schema mappers
 * deliberately: it mirrors their unwrapping onto the projected response
 * examples, which live under {@code paths} rather than
 * {@code components.schemas} and were therefore left wrapped. It runs first
 * (order 90), because the wrapper property name is only readable while the
 * schema is still wrapped.
 */
public final class BareArrayExtension implements Smithy2OpenApiExtension {

    @Override
    public List<OpenApiMapper> getOpenApiMappers() {
        return List.of(
                new BareResponseExampleMapper(),
                new BareArrayResponseMapper(),
                new BareObjectResponseMapper(),
                new MultipartRequestBodyMapper());
    }
}
