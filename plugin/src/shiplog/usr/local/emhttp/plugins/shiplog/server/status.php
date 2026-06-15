<?php
/* ShipLog same-origin proxy: the browser on the Docker tab calls this PHP, and
 * this PHP calls the local engine daemon on 127.0.0.1 — so the browser never
 * makes a cross-origin request (no CORS) and any token stays server-side.
 *
 *   GET status.php            -> engine /api/containers   (the whole fleet)
 *   GET status.php?id=<name>  -> engine /api/container/{id}
 */

header('Content-Type: application/json');

$cfg  = '/boot/config/plugins/shiplog/shiplog.cfg';
$port = '8484';
if (is_file($cfg)) {
    foreach (file($cfg) as $line) {
        if (preg_match('/^\s*PORT\s*=\s*"?([0-9]{1,5})"?/', $line, $m)) {
            $port = $m[1];
        }
    }
}

$id   = isset($_GET['id']) ? preg_replace('/[^A-Za-z0-9_.\-]/', '', $_GET['id']) : '';
$path = $id !== '' ? '/api/container/' . rawurlencode($id) : '/api/containers';
$url  = "http://127.0.0.1:$port$path";

$ch = curl_init($url);
curl_setopt_array($ch, [
    CURLOPT_RETURNTRANSFER => true,
    CURLOPT_CONNECTTIMEOUT => 3,
    CURLOPT_TIMEOUT        => 8,
]);
$body = curl_exec($ch);
$code = (int) curl_getinfo($ch, CURLINFO_HTTP_CODE);
curl_close($ch);

if ($body === false || $code === 0) {
    http_response_code(503);
    echo '{"error":"engine unreachable","port":"' . $port . '"}';
    exit;
}
http_response_code($code ?: 200);
echo $body;
