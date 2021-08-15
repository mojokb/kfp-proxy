FROM golang:1.16
COPY *.go .
RUN go build  -o /app main.go
EXPOSE 6996
CMD ["/app"]

