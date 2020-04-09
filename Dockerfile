FROM atsuio/alpine

# default port
EXPOSE 8040

COPY chatops /app/
COPY templates /data/templates

RUN mkdir -p /data/db && chown 1000 /data/db

WORKDIR /data

ENTRYPOINT ["/app/chatops"]