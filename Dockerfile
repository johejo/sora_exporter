FROM gcr.io/distroless/static-debian10
COPY sora_exporter /
ENTRYPOINT ["/sora_exporter"]
