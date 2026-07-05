FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /opt/clinepass-proxy
COPY clinepass-proxy .
RUN chmod +x clinepass-proxy

EXPOSE 55991

ENTRYPOINT ["./clinepass-proxy"]
CMD ["-host", "0.0.0.0", "-port", "55991", "-thinking-lang", "zh"]