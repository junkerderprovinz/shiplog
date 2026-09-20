<?php
/* Finds a reachable Ollama for the settings page: the common host ports first,
 * then the published ports and network IPs of a running ollama container, which
 * covers a container on br0 with its own IP and nothing published.
 * Returns {"ok":bool,"url":string,"models":[...],"message":string}. */

header('Content-Type: application/json');

/* Returns the model names at <base>/api/tags, or null. */
function ollamaTags($base)
{
    $ch = curl_init($base . '/api/tags');
    curl_setopt_array($ch, [
        CURLOPT_RETURNTRANSFER => true,
        CURLOPT_CONNECTTIMEOUT => 2,
        CURLOPT_TIMEOUT        => 4,
    ]);
    $body = curl_exec($ch);
    $code = (int) curl_getinfo($ch, CURLINFO_HTTP_CODE);
    curl_close($ch);
    if ($body === false || $code !== 200) {
        return null;
    }
    $data = json_decode($body, true);
    if (!is_array($data) || !isset($data['models'])) {
        return null;
    }
    $names = [];
    foreach ($data['models'] as $m) {
        if (isset($m['name'])) {
            $names[] = $m['name'];
        }
    }
    return $names;
}

/* GETs <path> from the Docker Engine over its unix socket. */
function dockerSock($path)
{
    $ch = curl_init('http://localhost' . $path);
    curl_setopt_array($ch, [
        CURLOPT_RETURNTRANSFER   => true,
        CURLOPT_UNIX_SOCKET_PATH => '/var/run/docker.sock',
        CURLOPT_CONNECTTIMEOUT   => 2,
        CURLOPT_TIMEOUT          => 5,
    ]);
    $body = curl_exec($ch);
    $code = (int) curl_getinfo($ch, CURLINFO_HTTP_CODE);
    curl_close($ch);
    return ($body !== false && $code === 200) ? json_decode($body, true) : null;
}

$hint   = isset($_GET['url']) ? trim($_GET['url']) : '';
$cands  = [];
$addr   = !empty($_SERVER['SERVER_ADDR']) ? $_SERVER['SERVER_ADDR'] : '';

if ($hint !== '' && preg_match('#^https?://#i', $hint)) {
    $cands[] = rtrim($hint, '/');
}
$cands[] = 'http://127.0.0.1:11434';
$cands[] = 'http://localhost:11434';
if ($addr !== '') {
    $cands[] = 'http://' . $addr . ':11434';
}
$cands[] = 'http://172.17.0.1:11434'; // docker0 gateway

$dockerSeen = false;
$list = dockerSock('/containers/json');
if (is_array($list)) {
    foreach ($list as $c) {
        $img = isset($c['Image']) ? $c['Image'] : '';
        if (!preg_match('/ollama/i', $img)) {
            continue;
        }
        $dockerSeen = true;
        if (!empty($c['Ports'])) {
            foreach ($c['Ports'] as $p) {
                if (isset($p['PrivatePort']) && (int) $p['PrivatePort'] === 11434 && !empty($p['PublicPort'])) {
                    $pp = (int) $p['PublicPort'];
                    $cands[] = 'http://127.0.0.1:' . $pp;
                    if ($addr !== '') {
                        $cands[] = 'http://' . $addr . ':' . $pp;
                    }
                }
            }
        }
        if (!empty($c['NetworkSettings']['Networks']) && is_array($c['NetworkSettings']['Networks'])) {
            foreach ($c['NetworkSettings']['Networks'] as $net) {
                if (!empty($net['IPAddress'])) {
                    $cands[] = 'http://' . $net['IPAddress'] . ':11434';
                }
            }
        }
    }
}

$seen = [];
foreach ($cands as $base) {
    if (isset($seen[$base])) {
        continue;
    }
    $seen[$base] = 1;
    $names = ollamaTags($base);
    if ($names === null) {
        continue;
    }
    echo json_encode([
        'ok'      => true,
        'url'     => $base,
        'models'  => $names,
        'message' => 'Found Ollama at ' . $base . ($names ? ', ' . count($names) . ' model(s).' : ', reachable, no models pulled yet.'),
    ]);
    exit;
}

$msg = $dockerSeen
    ? 'Found an Ollama container but could not reach its API on port 11434. Is the model server listening, and can Unraid route to its IP?'
    : 'No Ollama found (tried host ports and the Docker socket). Enter the URL manually.';
echo json_encode(['ok' => false, 'url' => '', 'models' => [], 'message' => $msg]);
