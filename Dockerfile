FROM golang:1.18 AS builder
WORKDIR /code
COPY go.* ./
RUN go mod download && go mod verify
COPY . ./
RUN git rev-parse --short HEAD && git rev-parse --symbolic-full-name --abbrev-ref HEAD # log buildID
RUN go build -ldflags="-w -s" .

FROM gcr.io/distroless/base:debug
COPY --from=builder /code/hdproxy /app/hdproxy
#make sure we got consistent current folder
WORKDIR /app
ENTRYPOINT [ "/app/hdproxy" ]
