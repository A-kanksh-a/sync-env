##
## Build binary
##
FROM golang:1.18-alpine AS build

WORKDIR /sync-env

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./

RUN export CGO_ENABLED=0 && go build -o ./sync-env

COPY ./sync-env /usr/local/bin
ENTRYPOINT ["sync-env"]


##
## RUN the binary
##
# FROM alpine
# COPY --from=build /sync-env /sync-env
# COPY /sync-env /usr/local/bin
# ENTRYPOINT ["sync-env"]
