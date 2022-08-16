ARG BUILDKIT_VERSION=v0.10.3-conflist
ARG CNI_VERSION=v1.1.1

FROM --platform=$BUILDPLATFORM alpine AS cni-plugins
RUN apk add --no-cache curl
ARG CNI_VERSION
ARG TARGETOS
ARG TARGETARCH
WORKDIR /opt/cni/bin
RUN curl -Ls https://github.com/containernetworking/plugins/releases/download/$CNI_VERSION/cni-plugins-$TARGETOS-$TARGETARCH-$CNI_VERSION.tgz | tar xzv

FROM vito/buildkit:${BUILDKIT_VERSION}
ARG BUILDKIT_VERSION
RUN apk add --no-cache iptables ip6tables
COPY --from=cni-plugins /opt/cni/bin /opt/cni/bin
ADD bass.conflist /etc/buildkit/bass.conflist
ADD buildkitd.toml /etc/buildkit/buildkitd.toml
