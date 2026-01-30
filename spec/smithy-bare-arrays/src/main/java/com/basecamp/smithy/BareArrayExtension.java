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
 */
public final class BareArrayExtension implements Smithy2OpenApiExtension {

    @Override
    public List<OpenApiMapper> getOpenApiMappers() {
        return List.of(new BareArrayResponseMapper(), new BareObjectResponseMapper());
    }
}
