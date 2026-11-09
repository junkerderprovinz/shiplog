<?php
/* Same-origin proxy from the WebGUI to the engine, which listens on a unix
 * socket and has no login of its own. Only a signed-in session reaches this
 * script, and Unraid checks the csrf_token of a POST before it runs, but not
 * of any other method, so every write comes in as a POST.
 *
 *   GET  status.php                                     -> /api/containers
 *   GET  status.php?id=<container id>                   -> /api/container/{id}
 *   GET  status.php?applied=<name>&digest=<sha256:...>  -> /api/applied
 *   GET  status.php?path=page[&mono=1]                  -> the status page
 *   GET  status.php?path=logo|overrides|unmaintained-suppressions
 *   GET  status.php?path=fortress-status[&lang=de]      -> the status contract
 *   GET  status.php?path=fortress-log[&after=<id>&limit=<n>&lang=de]
 *   POST status.php  path=refresh|override|unmaintained-suppression,
 *                    method=POST|PUT|DELETE, data=<JSON body>
 */

$sock   = '/var/run/shiplog.sock';
$self   = '/plugins/shiplog/server/status.php';
$reads  = ['page' => '/', 'logo' => '/logo.svg', 'overrides' => '/api/overrides',
           'unmaintained-suppressions' => '/api/unmaintained-suppressions',
           'fortress-status' => '/api/fortress/v1/status', 'fortress-log' => '/api/fortress/v1/log'];
$writes = ['refresh' => ['POST'], 'override' => ['PUT', 'DELETE'],
           'unmaintained-suppression' => ['PUT', 'DELETE']];

$deny = function ($code) {
    http_response_code($code);
    header('Content-Type: application/json');
    echo '{"error":"not allowed"}';
    exit;
};

$verb    = 'GET';
$headers = [];
$body    = null;
if ($_SERVER['REQUEST_METHOD'] === 'POST') {
    $path = isset($_POST['path']) ? (string) $_POST['path'] : '';
    $verb = isset($_POST['method']) ? (string) $_POST['method'] : '';
    if (!isset($writes[$path]) || !in_array($verb, $writes[$path], true)) {
        $deny(400);
    }
    $url       = '/api/' . $path;
    $body      = isset($_POST['data']) ? (string) $_POST['data'] : '';
    $headers[] = 'Content-Type: application/json';
} elseif ($_SERVER['REQUEST_METHOD'] === 'GET') {
    $path    = isset($_GET['path']) ? (string) $_GET['path'] : '';
    $id      = isset($_GET['id']) ? preg_replace('/[^0-9a-f]/', '', $_GET['id']) : '';
    $applied = isset($_GET['applied']) ? preg_replace('/[^A-Za-z0-9_.\-]/', '', $_GET['applied']) : '';
    $digest  = isset($_GET['digest']) ? preg_replace('/[^a-z0-9:]/', '', $_GET['digest']) : '';
    if ($path !== '') {
        if (!isset($reads[$path])) {
            $deny(400);
        }
        $url = $reads[$path];
        if ($path === 'fortress-status' || $path === 'fortress-log') {
            $q = [];
            if (isset($_GET['lang']) && preg_match('/^[A-Za-z]{1,8}(-[A-Za-z0-9]{1,8})*$/', $_GET['lang'])) {
                $q['lang'] = $_GET['lang'];
            } elseif (isset($_SERVER['HTTP_ACCEPT_LANGUAGE'])) {
                $headers[] = 'Accept-Language: ' . preg_replace('/[^A-Za-z0-9,;=.\- ]/', '', $_SERVER['HTTP_ACCEPT_LANGUAGE']);
            }
            if ($path === 'fortress-log') {
                foreach (['after', 'limit'] as $k) {
                    if (isset($_GET[$k]) && preg_match('/^\d{1,19}$/', $_GET[$k])) {
                        $q[$k] = $_GET[$k];
                    }
                }
            }
            if ($q) {
                $url .= '?' . http_build_query($q);
            }
        }
        if ($path === 'page') {
            // The page sends its calls back here, and its writes need the token.
            if (isset($_GET['mono']) && $_GET['mono'] === '1') {
                $url .= '?mono=1';
            }
            $var       = (array) @parse_ini_file('/var/local/emhttp/var.ini');
            $headers[] = 'X-ShipLog-Proxy: ' . $self;
            $headers[] = 'X-ShipLog-CSRF: ' . (isset($var['csrf_token']) ? $var['csrf_token'] : '');
        }
    } elseif ($applied !== '') {
        $url = '/api/applied?' . http_build_query(['name' => $applied, 'digest' => $digest]);
    } elseif ($id !== '') {
        $url = '/api/container/' . $id;
    } else {
        $url = '/api/containers';
    }
} else {
    $deny(405);
}

$ch = curl_init();
curl_setopt_array($ch, [
    CURLOPT_UNIX_SOCKET_PATH => $sock,
    CURLOPT_URL              => 'http://localhost' . $url,
    CURLOPT_CUSTOMREQUEST    => $verb,
    CURLOPT_HTTPHEADER       => $headers,
    CURLOPT_RETURNTRANSFER   => true,
    CURLOPT_CONNECTTIMEOUT   => 3,
    // Saving a source checks it against GitHub, which can take a few seconds.
    CURLOPT_TIMEOUT          => 20,
]);
if ($body !== null) {
    curl_setopt($ch, CURLOPT_POSTFIELDS, $body);
}
$resp = curl_exec($ch);
$code = (int) curl_getinfo($ch, CURLINFO_HTTP_CODE);
$type = (string) curl_getinfo($ch, CURLINFO_CONTENT_TYPE);
curl_close($ch);

if ($resp === false || $code === 0) {
    http_response_code(503);
    header('Content-Type: application/json');
    echo '{"error":"engine unreachable"}';
    exit;
}
http_response_code($code);
header('Content-Type: ' . ($type !== '' ? $type : 'application/json'));
echo $resp;
