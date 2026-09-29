# Backup runner (web-v1.md §15.2). Same PostgreSQL major as the database so pg_dump matches.
FROM postgres:18-alpine
RUN apk add --no-cache bash gnupg rclone coreutils findutils tar
COPY *.sh /ops/
RUN chmod 0755 /ops/*.sh
ENV GNUPGHOME=/tmp/gnupg
WORKDIR /work
ENTRYPOINT ["/ops/schedule.sh"]
