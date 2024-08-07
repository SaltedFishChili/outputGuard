FROM alpine:latest
USER root
RUN mkdir -p /data/logs && mkdir -p /apps
COPY ./router /apps/
WORKDIR /apps/
ENTRYPOINT ["./router"]