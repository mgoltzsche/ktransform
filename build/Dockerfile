FROM docker.io/library/golang:1.14-alpine3.12 AS builddeps
RUN apk add --update --no-cache curl make gcc libc-dev
COPY go.mod go.sum tools.go /go/src/github.com/mgoltzsche/ktransform/
WORKDIR /go/src/github.com/mgoltzsche/ktransform
RUN go mod download
RUN cat tools.go | grep -E '^\s*_' | cut -d'"' -f2 | xargs -n1 go install

FROM builddeps AS build
COPY version /go/src/github.com/mgoltzsche/ktransform/version
COPY pkg /go/src/github.com/mgoltzsche/ktransform/pkg
COPY cmd /go/src/github.com/mgoltzsche/ktransform/cmd
RUN go build -ldflags '-s -w -extldflags "-static"' ./cmd/manager && mv manager /bin/

# Prepare operator image
FROM alpine:3.12 AS operator
COPY build/bin /usr/local/bin
ENV OPERATOR=/usr/local/bin/ktransform \
    USER_UID=1001 \
    USER_NAME=ktransform
RUN /usr/local/bin/user_setup
USER ${USER_UID}
ENTRYPOINT ["/usr/local/bin/entrypoint"]
COPY --from=build /bin/manager ${OPERATOR}

FROM build AS test
RUN go test -v ./pkg/...

FROM operator