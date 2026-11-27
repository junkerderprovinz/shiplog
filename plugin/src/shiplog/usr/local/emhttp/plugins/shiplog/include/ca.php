<?php
/* Whether CA Auto Update Applications updates containers too. Its Docker part
   runs on the schedule saved in DockerUpdateSettings.json, and until that part
   is first applied there is no file and nothing runs. Settings that cannot be
   read count as updating: a warning too many costs less than a double update. */
return function ($plugin = '/usr/local/emhttp/plugins/ca.update.applications', $settings = '/boot/config/plugins/ca.update.applications/DockerUpdateSettings.json') {
  if (!is_dir($plugin) && !is_dir(dirname($settings))) return false;
  if (!file_exists($settings)) return false;
  $s = json_decode((string)@file_get_contents($settings), true);
  return !isset($s['cron']['dockerCronFrequency']) || $s['cron']['dockerCronFrequency'] !== 'disabled';
};
