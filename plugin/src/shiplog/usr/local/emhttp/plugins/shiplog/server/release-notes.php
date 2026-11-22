<?php
/* Answers What's new on the Info tab with {"version":...,"notes":...}, the
 * notes in GitHub's Markdown as include/notes.php reads them from the plg. */

header('Content-Type: application/json');

$notes = include '/usr/local/emhttp/plugins/shiplog/include/notes.php';
echo json_encode($notes(), JSON_UNESCAPED_UNICODE | JSON_UNESCAPED_SLASHES);
