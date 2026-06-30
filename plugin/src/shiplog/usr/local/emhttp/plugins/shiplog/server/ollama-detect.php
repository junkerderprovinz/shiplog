<?php
/* ShipLog — Ollama autodetect for the settings page. Finds a reachable Ollama
 * from the Unraid host: first the common host ports, then — crucially — it asks
 * the Docker socket for a running container whose image is "ollama" and probes
 * its real address (host-published port AND its per-network container IP), so it
 * works when Ollama runs on a custom/br0 bridge with its own IP and nothing is
 * published to the host. Server-side (no CORS).
 * Returns {"ok":bool,"url":string,"models":[...],"message":string}. */

header('Content-Type: application/json');

/* GET <base>/api/tags; returns the model-name array on HTTP 200, else null. */
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

/* GET <path> from the Docker Engine over its unix socket (read-only use here). */
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

// 1) Cheap host-port guesses first.
if ($hint !== '' && preg_match('#^https?://#i', $hint)) {
    $cands[] = rtrim($hint, '/');
}
$cands[] = 'http://127.0.0.1:11434';
$cands[] = 'http://localhost:11434';
if ($addr !== '') {
    $cands[] = 'http://' . $addr . ':11434';
}
$cands[] = 'http://172.17.0.1:11434'; // docker0 gateway

// 2) Ask Docker for a running "ollama" container and add its real addresses.
$dockerSeen = false;
$list = dockerSock('/containers/json');
if (is_array($list)) {
    foreach ($list as $c) {
        $img = isset($c['Image']) ? $c['Image'] : '';
        if (!preg_match('/ollama/i', $img)) {
            continue;
        }
        $dockerSeen = true;
        // host-published mapping of the Ollama port
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
        // the container's own IP on each network it is attached to
        if (!empty($c['NetworkSettings']['Networks']) && is_array($c['NetworkSettings']['Networks'])) {
            foreach ($c['NetworkSettings']['Networks'] as $net) {
                if (!empty($net['IPAddress'])) {
                    $cands[] = 'http://' . $net['IPAddress'] . ':11434';
                }
            }
        }
    }
}

// Probe candidates in order, first reachable wins.
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
        'message' => 'Found Ollama at ' . $base . ($names ? ' — ' . count($names) . ' model(s).' : ' — reachable, no models pulled yet.'),
    ]);
    exit;
}

$msg = $dockerSeen
    ? 'Found an Ollama container but could not reach its API on port 11434 — is the model server listening, and can Unraid route to its IP?'
    : 'No Ollama found (tried host ports and the Docker socket). Enter the URL manually.';
echo json_encode(['ok' => false, 'url' => '', 'models' => [], 'message' => $msg]);
