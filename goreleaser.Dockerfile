FROM scratch
ARG TARGETPLATFORM
ENTRYPOINT [ "/manager" ]
USER 65532:65532
COPY ${TARGETPLATFORM}/manager /manager

LABEL org.opencontainers.image.source="https://github.com/alphagov/govuk-job-request-operator"
