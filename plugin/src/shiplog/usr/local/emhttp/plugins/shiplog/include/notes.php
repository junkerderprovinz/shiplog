<?php
/* The release notes What's new shows: the block of the running version in the
   CHANGES of the installed plg, or of the new version in the plg `plugin check`
   fetched while that one waits in the Plugins tab. The release page is cut from
   the same block, with its category lines turned into headings, so the window
   reads what GitHub shows. A build that is not a release reads the newest block. */
return function ($installed = '/var/log/plugins/shiplog.plg', $fetched = '/tmp/plugins/shiplog.plg') {
  $U = include __DIR__.'/update.php';
  $state = $U($installed, $fetched);
  $plg = (string)@file_get_contents($state['kind'] === 'available' ? $fetched : $installed);
  $version = preg_match('/<!ENTITY\s+version\s+"([^"]+)"/', $plg, $m) ? $m[1] : '';
  $blocks = preg_match('/<CHANGES>(.*?)<\/CHANGES>/s', $plg, $c)
    ? preg_split('/^### +(\S+)[ \t]*$/m', html_entity_decode($c[1], ENT_QUOTES | ENT_XML1, 'UTF-8'), -1, PREG_SPLIT_DELIM_CAPTURE)
    : array();
  $release = preg_match('/^\d+(\.\d+)+$/', $version);
  for ($i = 1; $i + 1 < count($blocks); $i += 2) {
    if ($release && $blocks[$i] !== $version) continue;
    $notes = preg_replace('/^(?!-)(\S+) \*\*([^*]+)\*\*[ \t]*$/mu', '## $1 $2', trim($blocks[$i + 1]));
    return array('version' => $blocks[$i], 'notes' => $notes);
  }
  return array('version' => $release ? $version : '', 'notes' => '');
};
