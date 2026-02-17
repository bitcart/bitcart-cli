FROM scratch
ARG TARGETPLATFORM
ENTRYPOINT [ "/usr/bin/bitcart-cli" ]
COPY $TARGETPLATFORM/bitcart-cli /usr/bin/bitcart-cli
