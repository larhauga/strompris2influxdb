FROM golang:1.17 
WORKDIR /src
RUN CGO_ENABLED=0 GOOS=linux go build .

FROM alpine:latest
WORKDIR /app
COPY --from=0 /src/strompris /app/strompris

CMD /app/strompris