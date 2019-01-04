FROM pivotalcfreleng/golang
MAINTAINER https://github.com/pivotal-cf/p-runtime

WORKDIR /tmp
RUN wget https://github.com/pivotal-cf/kiln/releases/download/0.17.0/kiln-linux &&\
  chmod +x kiln-linux &&\
  mv kiln-linux /usr/local/bin/kiln
