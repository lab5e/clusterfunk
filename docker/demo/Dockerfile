FROM alpine
LABEL Author="Ståle Dahl <stalehd@gmail.com>"
LABEL Description="Demo service for the clusterfunk library"
RUN apk --update upgrade
# See http://stackoverflow.com/questions/34729748/installed-go-binary-not-found-in-path-on-alpine-linux-docker
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2

# HTTP server
EXPOSE 8080/tcp
# Management
EXPOSE 1234/tcp

ADD demo.linux /demo
ADD run-demo.sh /run-demo.sh

CMD ["/bin/sh", "/run-demo.sh"]
