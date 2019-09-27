FROM alpine:latest
RUN apk update
RUN apk add -u bash 

COPY Dockerfile /cnab/Dockerfile
COPY app /cnab/app
RUN chmod +x /cnab/app/run
CMD ["/cnab/app/run"]