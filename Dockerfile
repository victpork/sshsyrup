FROM golang:1.11 AS builder

# Copy the code from the host and compile it
WORKDIR /syrup
ENV GO111MODULE=on
COPY . ./
RUN go get ./...
RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags "-s -w" -installsuffix nocgo -o /sshsyrup ./cmd/syrup
RUN ssh-keygen -t rsa -q -f id_rsa -N "" && cp id_rsa id_rsa.pub /
RUN cp -r commands.txt config.yaml group passwd filesystem.zip cmdOutput /

FROM scratch
COPY --from=builder /config.yaml ./
COPY --from=builder /filesystem.zip ./
COPY --from=builder /group ./
COPY --from=builder /passwd ./
COPY --from=builder /id_rsa ./
COPY --from=builder /commands.txt ./
COPY --from=builder /sshsyrup ./
COPY --from=builder /cmdOutput ./cmdOutput
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

ENTRYPOINT ["./sshsyrup"]

EXPOSE 22/tcp
