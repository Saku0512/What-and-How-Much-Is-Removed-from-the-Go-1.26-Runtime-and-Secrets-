FROM golang:1.26-bookworm

RUN apt-get update \
    && apt-get install -y --no-install-recommends gdb \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /src
COPY . .

CMD ["make", "validate"]
