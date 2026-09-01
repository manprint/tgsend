FROM gcr.io/distroless/static-debian12:nonroot

ARG TARGETPLATFORM

LABEL org.opencontainers.image.title="tgsend" \
      org.opencontainers.image.description="Send exact UTF-8 messages to Telegram" \
      org.opencontainers.image.source="https://github.com/manprint/tgsend" \
      org.opencontainers.image.licenses="MIT"

COPY ${TARGETPLATFORM}/tgsend /usr/local/bin/tgsend

USER 65532:65532
ENTRYPOINT ["/usr/local/bin/tgsend"]
