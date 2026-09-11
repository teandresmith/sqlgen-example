# GraphQL is the only API surface

The example exposes a GraphQL API (sqlgen's gqlgen-based generation) and no REST
or gRPC. Two reasons: GraphQL is sqlgen's mature, fully-supported API path today —
`api.rest` and `api.grpc` config structs exist but generate no code yet — and a
single, coherent "blessed path" is more valuable to an adopter than a broader but
shallower surface. If REST is added later it should be a clearly-secondary
follow-up, not a co-equal surface that dilutes the example.
