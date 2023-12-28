FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y ca-certificates
ADD nyaa /nyaa
WORKDIR /
ENTRYPOINT ["/nyaa"]
EXPOSE 3333
CMD ["-f", "/conf.yml"]
