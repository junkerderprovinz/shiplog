<?php
/* Where ShipLog's updates stand, read the way Unraid's Plugins tab reads it:
   the plg that `plugin check` fetched into /tmp/plugins against the installed
   one, compared with strcmp as Unraid compares them, so the Info tab never
   offers an update the Plugins tab does not show. Until the first check since
   boot there is nothing to compare. */
return function ($installed = '/var/log/plugins/shiplog.plg', $fetched = '/tmp/plugins/shiplog.plg') {
  $version = function ($plg) {
    return preg_match('/<!ENTITY\s+version\s+"([^"]+)"/', (string)@file_get_contents($plg), $m) ? $m[1] : '';
  };
  if (!is_file($fetched)) return array('kind' => 'unchecked');
  $new = $version($fetched);
  if (strcmp($new, $version($installed)) > 0) return array('kind' => 'available', 'version' => $new);
  return array('kind' => 'current', 'checkedAt' => filemtime($fetched));
};
