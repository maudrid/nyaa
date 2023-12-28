FROM debian:bookworm-slim
ADD nyaa /nyaa
WORKDIR /
ENTRYPOINT ["/nyaa"]
EXPOSE 3333
CMD ["-f", "/conf.yml"]
