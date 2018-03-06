FROM alpine:3.7

ADD ./async /usr/bin

CMD ["/usr/bin/async"]
