FROM scratch
MAINTAINER KASKO Ltd, sysarch@kasko.io

ADD dist/cacert.pem /etc/ssl/ca-bundle.pem
ADD dist/vault-monitor-linux-amd64 /bin/vault-monitor

ENV PATH=/bin
ENV TMPDIR=/

CMD ["/bin/vault-monitor"]
