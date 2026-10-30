# The vigie image: the vigie binary alone, reading /etc/vigie/config.yml.

FROM golang:1.27 AS build
ARG VERSION=dev
ARG COMMIT=none
ARG DATE=unknown
ENV CGO_ENABLED=0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.date=${DATE}" -o /out/vigie .

# static keeps the ca certificates the url sources need to check https servers
FROM gcr.io/distroless/static-debian13:nonroot
COPY --from=build /out/vigie /usr/local/bin/vigie
EXPOSE 8080
ENTRYPOINT ["vigie"]
CMD ["serve", "-c", "/etc/vigie/config.yml"]
