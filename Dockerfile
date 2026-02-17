FROM scratch
ENTRYPOINT [ "/usr/bin/bitcart-cli" ]
COPY bitcart-cli /usr/bin/bitcart-cli
