FROM alpine:latest
USER root
RUN mkdir -p /data/logs && mkdir -p /apps
COPY ./server /apps/server
WORKDIR /apps/server
ENTRYPOINT ["./server"]