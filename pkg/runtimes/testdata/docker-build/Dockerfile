FROM alpine AS default
WORKDIR /wd
RUN echo hello from Dockerfile > wd-file

FROM alpine AS alt
WORKDIR /wd
RUN echo hello from alt stage in Dockerfile > wd-file

FROM alpine AS arg
WORKDIR /wd
ARG MESSAGE
RUN echo hello from Dockerfile with message $MESSAGE > wd-file

FROM alpine AS env
WORKDIR /wd
ENV FOO=bar
RUN echo hello from Dockerfile with env $FOO > wd-file

FROM default
