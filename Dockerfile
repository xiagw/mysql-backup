# mysql backup image
FROM alpine:3.18

# If you're in China, set IN_CHINA to true.
ARG IN_CHINA=true

COPY ./root/build.sh /
RUN sh /build.sh

COPY ./root/ /

USER appuser

# start
# ENTRYPOINT ["/run.sh"]

ENTRYPOINT [ "/usr/bin/dumb-init", "--" ]
CMD [ "/run.sh" ]