FROM golang:1.12.3 as build
ENV GO111MODULE=on
WORKDIR /go/src/app

# download modules first; this is slow and its source changes infrequently,
# so we want its results cached to get to the build step faster on rebuilds
COPY go.mod .
COPY go.sum .
RUN go mod download

# build the app
COPY . .
RUN go build -o /go/bin/app ./cmd/k8s-api-coverage-server 

FROM gcr.io/distroless/base:latest
COPY --from=build /go/bin/app /
COPY --from=build /go/src/app/ignoredfields.yaml /
CMD ["/app"]
