FROM golang:1.17 
WORKDIR /src
COPY main.go go.mod go.sum /src/
RUN CGO_ENABLED=0 GOOS=linux go build -o strompris .

FROM alpine:latest
WORKDIR /app
COPY --from=0 /src/strompris /app/strompris
CMD /app/strompris