##
## Build binary
##
FROM golang:1.18-alpine AS build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 go build

##
## RUN the binary
##

FROM alpine

COPY --from=build /app/sync-env /usr/local/bin

#USER root:root

ENTRYPOINT ["sync-env"]
