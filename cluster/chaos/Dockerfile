# The node binary is built by the test harness (GOOS=linux), so the image needs no build tools.
FROM alpine:3.20
RUN apk add --no-cache iptables iproute2
COPY bin/mochi-cluster /mochi-cluster
ENTRYPOINT ["/mochi-cluster"]
