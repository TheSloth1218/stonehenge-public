const patch = require("../../../stonehenge-middleware/docker/fuxa/patch-fuxa-main");

if (require.main === module) {
  patch.patchServer();
}

module.exports = patch;
