FROM debian

RUN apt-get update && apt-get install -y man

RUN mkdir -p /walk
WORKDIR /
COPY . /walk

RUN dpkg-deb --build walk && \
    dpkg -i walk.deb && \
    man -P cat walk
