FROM frangoteam/fuxa:1.3.3

WORKDIR /usr/src/app/FUXA/server

RUN npm install --no-audit --no-fund \
    mcprotocol@0.1.2 \
    modbus-serial@8.0.19 \
    node-red@4.1.10
COPY docker/fuxa/stonehenge-hmi-proxy.js ./api/stonehenge-hmi-proxy.js
COPY docker/fuxa/patch-fuxa-main.js /tmp/patch-fuxa-main.js
RUN node /tmp/patch-fuxa-main.js && rm /tmp/patch-fuxa-main.js
