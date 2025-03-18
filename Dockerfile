FROM golang:1.22.2 AS build


WORKDIR /src
COPY . .
RUN go env -w GOPROXY=https://goproxy.cn,direct
RUN STATIC=0 GOOS=linux GOARCH=amd64 LDFLAGS='-extldflags -static -s -w' go build -o nacos-bench cmd/main/main.go 

FROM ubuntu:24.04
RUN apt-get update && \
    apt-get install -y net-tools && \
    apt install curl -y && \
    apt install telnet -y && \
    apt install vim -y && \
    apt install less -y && \
    rm -rf /var/lib/apt/lists/*
WORKDIR /
COPY --from=build /src/nacos-bench /nacos-bench
CMD ["./nacos-bench"]