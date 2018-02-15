FROM golang:1.9 AS builder

# Download and install the latest release of dep
ADD https://github.com/golang/dep/releases/download/v0.4.1/dep-linux-amd64 /usr/bin/dep
RUN chmod +x /usr/bin/dep

# Copy the code from the host and compile it
WORKDIR $GOPATH/src/github.com/mkishere/sshsyrup
COPY Gopkg.toml Gopkg.lock ./
RUN dep ensure --vendor-only
COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags "-s -w" -installsuffix nocgo -o /sshsyrup ./cmd/syrup
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags "-s -w" -installsuffix nocgo -o /createfs ./cmd/createfs
RUN ssh-keygen -t rsa -q -f id_rsa -N "" && cp id_rsa id_rsa.pub /
RUN cp -r config.json group passwd filesystem.zip logs /

FROM scratch
COPY --from=builder /config.json ./
COPY --from=builder /filesystem.zip ./
COPY --from=builder /group ./
COPY --from=builder /passwd ./
COPY --from=builder /id_rsa.pub ./
COPY --from=builder /id_rsa ./
COPY --from=builder /sshsyrup ./
COPY --from=builder /createfs ./
COPY --from=builder /logs ./logs
ENTRYPOINT ["./sshsyrup"]
