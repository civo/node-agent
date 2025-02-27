FROM alpine 

COPY node-agent /usr/bin

ENTRYPOINT ["/usr/bin/node-agent"]
