subinclude("///third_party/subrepos/pleasings//docker")

go_binary(
    name = "prometheus-cardinality-exporter",
    srcs = ["main.go"],
    static=True,
    deps = [
        "//cardinality",
        "//third_party/go:prometheus",
        "//third_party/go/kubernetes:apimachinery",
        "//third_party/go/kubernetes:client-go",
        "//third_party/go:logrus",
        "//third_party/go:backoff",
        "//third_party/go:go-flags",
        "//third_party/go:yaml.v3",
    ],
)

docker_image(
    name = "prometheus-cardinality-exporter_alpine",
    srcs = [
        ":prometheus-cardinality-exporter",
    ],
    labels = ["docker"],
    dockerfile = "Dockerfile-prometheus-cardinality-exporter",
    image = "prometheus-cardinality-exporter",
    visibility = [
        "//k8s",
    ],
)

docker_image(
    name = "prometheus-cardinality-exporter_distroless",
    srcs = [
        ":prometheus-cardinality-exporter",
    ],
    dockerfile = "Dockerfile-prometheus-cardinality-exporter_distroless",
    labels = ["docker"],
    image = "prometheus-cardinality-exporter_distroless",
    visibility = [
        "//k8s",
    ],
)
