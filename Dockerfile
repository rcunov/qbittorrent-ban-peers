FROM alpine:3.20

# Install dependencies
RUN apk add --no-cache \
    curl \
    jq \
    bash

# Create "sidecar" user
RUN adduser --system --home /home/sidecar --shell /bin/bash --disabled-password sidecar

# Run as "sidecar" user
USER sidecar

WORKDIR /home/sidecar

COPY banhammer.sh .

ENTRYPOINT ["/bin/bash"]

CMD ["banhammer.sh"]
