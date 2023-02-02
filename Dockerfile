FROM alpine:3.17

COPY ./bitcart-cli /usr/local/bin

RUN apk add --no-cache --upgrade ca-certificates

ENTRYPOINT ["bitcart-cli"]
