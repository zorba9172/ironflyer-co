# IronFlyer slim OpenVSCode Server.
#
# This is the default cloud IDE used by the Studio iframe. It keeps the
# upstream OpenVSCode runtime, but bakes in the same slim IronFlyer
# settings profile used by the legacy code-server image.
FROM gitpod/openvscode-server:latest

USER root
RUN mkdir -p /home/.openvscode-server/data/User \
 && chown -R openvscode-server:openvscode-server /home/.openvscode-server
COPY infra/docker/ironflyer-code/settings.json /home/.openvscode-server/data/User/settings.json
COPY infra/docker/ironflyer-code/keybindings.json /home/.openvscode-server/data/User/keybindings.json
RUN chown openvscode-server:openvscode-server \
      /home/.openvscode-server/data/User/settings.json \
      /home/.openvscode-server/data/User/keybindings.json

USER openvscode-server
