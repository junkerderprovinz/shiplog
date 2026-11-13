/*
 * The fortress card for a page without a framework, such as an Unraid
 * plugin's settings ("The fortress" in the design language). One call draws
 * it into an element: the card with joining by the phrase, the way out and
 * the switch for the other apps, then a card per other app with Remove, and
 * Open for a plugin, which shows its status in a window. Everything that
 * differs between the apps comes in through the adapter, which talks to the
 * app's own server; the card owns the drawing, the windows, the texts in
 * every language GlimStone carries, and keeping the state fresh.
 *
 * Plugins copy this file and fortress-card.css unchanged.
 */
(function (root) {
  "use strict";

  /* texts:begin */
  const TEXTS = {
    "en": {"fortress":"Fortress","showOthers":"Show the other apps of the fortress","about":"In a fortress your apps see each other and take work off each other's hands. An app gets in with the phrase.","othersApps":{"one":"{count} more app","other":"{count} more apps"},"othersDevices":{"one":"{count} device","other":"{count} devices"},"othersLine":"{apps} on {devices}. Applies to this device only.","connected":"Connected","away":"Not reachable","viaRelay":"Via relay","lastSeen":"Last seen {time}","instanceCount":{"one":"{count} instance","other":"{count} instances"},"forms":{"container":"Container","plugin":"Unraid plugin","desktop":"Desktop app","android":"Android app","extension":"Browser extension"},"open":"Open","remove":"Remove","memberOn":"{app} on {device}","outHint":"Join a fortress and your other instances of {app} show up here, along with the other apps if you like.","removedHereTitle":"Removed from the fortress","removedHereHint":"Another app in the fortress removed {app}. Enter the current phrase to come back in.","enterPhrase":"Enter phrase","joinTitle":"Join a fortress","word":"Word {n}","phraseWhere":"Every app in the fortress shows the phrase under Instances. Paste it into the first field and it spreads over the rest.","join":"Join","joinRefused":"These words open no fortress. Check them and try again.","joinedTitle":"{app} is in the fortress now","cancel":"Cancel","close":"Close","leaveTitle":"Leave the fortress?","leaveBody":"The other apps won't see {app} anymore, and every permission with {app} ends.","leaveBack":"The current phrase brings you back.","leaveConfirm":"Leave","removeTitle":"Remove {device}?","removeBody":"{device} loses its access right away. This can't be undone.","removeBack":"Anyone who still has the phrase can get back in until you change it.","removedTitle":"Change the phrase now?","removedBody":"{device} is out. As long as the phrase stays the same, the way back in is still open.","removedNote":"Removed: {devices}. Change the phrase to close the way back in.","phraseTitle":"Phrase","leaveRow":"This app in the fortress","leaveAbout":"Takes {app} out of the fortress. The current phrase brings it back.","changedTitle":"New phrase","changeElsewhere":"You change the phrase in {apps}.","changeNowhere":"{app} doesn't keep the phrase. Change it in an app of your fortress that shows it.","phraseMoved":"Your fortress has a new phrase that {app} couldn't follow on its own. Enter the new words and it stays the same member.","failed":"That didn't work. Please try again.","statusWhere":"{app} keeps its settings in Unraid on {device}.","statusLoading":"Asking {app} how it's doing.","statusQuiet":"Nothing needs you right now, and nothing is running.","statusNeedsYou":"Needs you","statusWork":"Running now","statusNext":"Coming up","statusLog":"Latest entries","states":{"off":"Off","ok":"All good","starting":"Starting","warning":"Needs a look","overdue":"Overdue","failed":"Failed"},"levels":{"warn":"Warning","error":"Error"}},
    "de": {"fortress":"Festung","showOthers":"Andere Apps der Festung zeigen","about":"In einer Festung sehen sich deine Apps und nehmen einander Arbeit ab. Hinein kommt eine App mit der Phrase.","othersApps":{"one":"{count} weitere App","other":"{count} weitere Apps"},"othersDevices":{"one":"{count} Gerät","other":"{count} Geräten"},"othersLine":"{apps} auf {devices}. Gilt nur für dieses Gerät.","connected":"Verbunden","away":"Nicht erreichbar","viaRelay":"Über Relay","lastSeen":"Zuletzt gesehen {time}","instanceCount":{"one":"{count} Instanz","other":"{count} Instanzen"},"forms":{"container":"Container","plugin":"Unraid-Plugin","desktop":"Desktop-App","android":"Android-App","extension":"Browser-Erweiterung"},"open":"Öffnen","remove":"Herausnehmen","memberOn":"{app} auf {device}","outHint":"Tritt einer Festung bei, dann stehen hier deine anderen Instanzen von {app} und auf Wunsch die übrigen Apps.","removedHereTitle":"Aus der Festung herausgenommen","removedHereHint":"Eine andere App der Festung hat {app} herausgenommen. Mit der aktuellen Phrase kommst du wieder hinein.","enterPhrase":"Phrase eingeben","joinTitle":"Festung beitreten","word":"Wort {n}","phraseWhere":"Die Phrase steht in jeder App der Festung unter Instanzen. Füg sie ins erste Feld ein, dann verteilt sie sich auf die anderen.","join":"Beitreten","joinRefused":"Diese Wörter öffnen keine Festung. Prüf sie und versuch es noch einmal.","joinedTitle":"{app} ist jetzt in der Festung","cancel":"Abbrechen","close":"Schließen","leaveTitle":"Festung verlassen?","leaveBody":"Die anderen Apps sehen {app} danach nicht mehr, und alle Erlaubnisse mit {app} enden.","leaveBack":"Zurück kommst du mit der aktuellen Phrase.","leaveConfirm":"Verlassen","removeTitle":"{device} herausnehmen?","removeBody":"{device} verliert sofort den Zugang. Das lässt sich nicht rückgängig machen.","removeBack":"Wer die Phrase noch kennt, kommt wieder hinein, bis du sie änderst.","removedTitle":"Phrase jetzt ändern?","removedBody":"{device} ist draußen. Solange die Phrase gleich bleibt, steht der Weg zurück aber noch offen.","removedNote":"Herausgenommen: {devices}. Ändere die Phrase, dann ist der Weg zurück zu.","phraseTitle":"Phrase","leaveRow":"Diese App in der Festung","leaveAbout":"Nimmt {app} aus der Festung. Mit der aktuellen Phrase kommt sie zurück.","changedTitle":"Neue Phrase","changeElsewhere":"Die Phrase änderst du in {apps}.","changeNowhere":"{app} merkt sich die Phrase nicht. Ändere sie in einer App der Festung, die sie anzeigt.","phraseMoved":"Deine Festung hat eine neue Phrase, der {app} nicht von selbst folgen konnte. Gib die neuen Wörter ein, dann bleibt das Mitglied dasselbe wie bisher.","failed":"Das hat nicht geklappt. Versuch es bitte noch einmal.","statusWhere":"Die Einstellungen von {app} liegen in Unraid auf {device}.","statusLoading":"Einen Moment, der Zustand von {app} kommt gleich.","statusQuiet":"Gerade braucht dich nichts, und es läuft nichts.","statusNeedsYou":"Braucht dich","statusWork":"Läuft gerade","statusNext":"Als Nächstes","statusLog":"Letzte Einträge","states":{"off":"Aus","ok":"Alles gut","starting":"Startet","warning":"Braucht einen Blick","overdue":"Überfällig","failed":"Fehlgeschlagen"},"levels":{"warn":"Warnung","error":"Fehler"}},
    "ar": {"fortress":"الحصن","showOthers":"عرض التطبيقات الأخرى في الحصن","about":"في الحصن ترى تطبيقاتك بعضها بعضًا وتتقاسم العمل فيما بينها. يدخل التطبيق إلى الحصن بالعبارة.","othersApps":{"zero":"{count} تطبيق آخر","one":"{count} تطبيق آخر","two":"{count} تطبيقان آخران","few":"{count} تطبيقات أخرى","many":"{count} تطبيقًا آخر","other":"{count} تطبيق آخر"},"othersDevices":{"zero":"{count} جهاز","one":"{count} جهاز","two":"{count} جهازين","few":"{count} أجهزة","many":"{count} جهازًا","other":"{count} جهاز"},"othersLine":"{apps} على {devices}. ينطبق على هذا الجهاز فقط.","connected":"متصل","away":"غير قابل للوصول","viaRelay":"عبر مُرحِّل","lastSeen":"آخر ظهور {time}","instanceCount":{"zero":"{count} مثيل","one":"{count} مثيل","two":"{count} مثيلان","few":"{count} مثيلات","many":"{count} مثيلًا","other":"{count} مثيل"},"forms":{"container":"حاوية","plugin":"إضافة Unraid","desktop":"تطبيق سطح المكتب","android":"تطبيق Android","extension":"إضافة المتصفح"},"open":"فتح","remove":"إزالة","memberOn":"{app} على {device}","outHint":"انضم إلى حصن وستظهر هنا مثيلاتك الأخرى من {app}، ومعها التطبيقات الأخرى إن أردت.","removedHereTitle":"أُزيل من الحصن","removedHereHint":"أزال تطبيق آخر في الحصن {app}. أدخل العبارة الحالية لتعود إليه.","enterPhrase":"إدخال العبارة","joinTitle":"الانضمام إلى حصن","word":"الكلمة {n}","phraseWhere":"يعرض كل تطبيق في الحصن العبارة ضمن «المثيلات». الصقها في الحقل الأول وستتوزع على بقية الحقول.","join":"انضمام","joinRefused":"هذه الكلمات لا تفتح أي حصن. تحقق منها وحاول مرة أخرى.","joinedTitle":"{app} في الحصن الآن","cancel":"إلغاء","close":"إغلاق","leaveTitle":"هل تريد مغادرة الحصن؟","leaveBody":"لن ترى التطبيقات الأخرى {app} بعد الآن، وتنتهي كل الأذونات مع {app}.","leaveBack":"يمكنك العودة بالعبارة الحالية.","leaveConfirm":"مغادرة","removeTitle":"هل تريد إزالة {device}؟","removeBody":"يفقد {device} وصوله فورًا. لا يمكن التراجع عن ذلك.","removeBack":"يمكن لأي شخص لا تزال لديه العبارة أن يعود حتى تغيّرها.","removedTitle":"هل تريد تغيير العبارة الآن؟","removedBody":"{device} خارج الحصن الآن. لكن ما دامت العبارة كما هي، يبقى طريق العودة مفتوحًا.","removedNote":"أُزيل: {devices}. غيّر العبارة لتغلق طريق العودة.","phraseTitle":"العبارة","leaveRow":"هذا التطبيق في الحصن","leaveAbout":"يُخرج {app} من الحصن. وتعيده العبارة الحالية.","changedTitle":"عبارة جديدة","changeElsewhere":"تغيّر العبارة في {apps}.","changeNowhere":"لا يحتفظ {app} بالعبارة. غيّرها في تطبيق من الحصن يعرضها.","phraseMoved":"لحصنك عبارة جديدة لم يتمكن {app} من اتباعها وحده. أدخل الكلمات الجديدة ليبقى العضو نفسه.","failed":"لم ينجح ذلك. يُرجى المحاولة مرة أخرى.","statusWhere":"يحفظ {app} إعداداته في Unraid على {device}.","statusLoading":"نسأل {app} عن حاله.","statusQuiet":"لا شيء يحتاجك الآن، ولا شيء قيد التشغيل.","statusNeedsYou":"يحتاجك","statusWork":"قيد التشغيل الآن","statusNext":"التالي","statusLog":"أحدث الإدخالات","states":{"off":"متوقف","ok":"كل شيء على ما يرام","starting":"قيد البدء","warning":"يحتاج إلى نظرة","overdue":"متأخر","failed":"فشل"},"levels":{"warn":"تحذير","error":"خطأ"}},
    "bg": {"fortress":"Крепост","showOthers":"Показване на другите приложения в крепостта","about":"В крепостта приложенията ви се виждат едно друго и си поемат част от работата. Приложение влиза в нея с фразата.","othersApps":{"one":"{count} друго приложение","other":"{count} други приложения"},"othersDevices":{"one":"{count} устройство","other":"{count} устройства"},"othersLine":"{apps} на {devices}. Важи само за това устройство.","connected":"Свързано","away":"Недостъпно","viaRelay":"Чрез препредавател","lastSeen":"Последно видяно {time}","instanceCount":{"one":"{count} инстанция","other":"{count} инстанции"},"forms":{"container":"Контейнер","plugin":"Добавка за Unraid","desktop":"Настолно приложение","android":"Приложение за Android","extension":"Разширение за браузър"},"open":"Отваряне","remove":"Премахване","memberOn":"{app} на {device}","outHint":"Присъединете се към крепост и тук ще се появят другите ви инстанции на {app}, а ако искате, и останалите приложения.","removedHereTitle":"Премахнато от крепостта","removedHereHint":"Друго приложение в крепостта премахна {app}. Въведете текущата фраза, за да влезете отново.","enterPhrase":"Въвеждане на фраза","joinTitle":"Присъединяване към крепост","word":"Дума {n}","phraseWhere":"Всяко приложение в крепостта показва фразата в раздел Инстанции. Поставете я в първото поле и тя сама ще се разпредели в останалите.","join":"Присъединяване","joinRefused":"Тези думи не отварят крепост. Проверете ги и опитайте отново.","joinedTitle":"{app} вече е в крепостта","cancel":"Отказ","close":"Затваряне","leaveTitle":"Напускане на крепостта?","leaveBody":"Другите приложения вече няма да виждат {app} и всички разрешения с {app} ще отпаднат.","leaveBack":"Можете да се върнете с текущата фраза.","leaveConfirm":"Напускане","removeTitle":"Премахване на {device}?","removeBody":"{device} губи достъпа си веднага. Това не може да бъде отменено.","removeBack":"Всеки, който още има фразата, може да се върне, докато не я смените.","removedTitle":"Смяна на фразата сега?","removedBody":"{device} е навън. Докато фразата остава същата, пътят обратно е още отворен.","removedNote":"Премахнати: {devices}. Сменете фразата, за да затворите пътя обратно.","phraseTitle":"Фраза","leaveRow":"Това приложение в крепостта","leaveAbout":"Изважда {app} от крепостта. Текущата фраза го връща.","changedTitle":"Нова фраза","changeElsewhere":"Фразата се сменя в {apps}.","changeNowhere":"{app} не пази фразата. Сменете я в приложение от крепостта, което я показва.","phraseMoved":"Крепостта ви има нова фраза, която {app} не успя да последва сам. Въведете новите думи и той остава същият член.","failed":"Не се получи. Опитайте отново.","statusWhere":"{app} пази настройките си в Unraid на {device}.","statusLoading":"Питаме {app} как е.","statusQuiet":"В момента нищо не изисква намесата ви и нищо не се изпълнява.","statusNeedsYou":"Чака ви","statusWork":"Изпълнява се сега","statusNext":"Предстои","statusLog":"Последни записи","states":{"off":"Изключено","ok":"Всичко е наред","starting":"Стартира","warning":"Нужна е проверка","overdue":"Просрочено","failed":"Неуспешно"},"levels":{"warn":"Предупреждение","error":"Грешка"}},
    "ca": {"fortress":"Fortalesa","showOthers":"Mostra les altres apps de la fortalesa","about":"En una fortalesa, les teves apps es veuen entre elles i es reparteixen la feina. Una app hi entra amb la frase.","othersApps":{"one":"{count} app més","many":"{count} apps més","other":"{count} apps més"},"othersDevices":{"one":"{count} dispositiu","many":"{count} dispositius","other":"{count} dispositius"},"othersLine":"{apps} en {devices}. Només s'aplica a aquest dispositiu.","connected":"Connectat","away":"No accessible","viaRelay":"Via repetidor","lastSeen":"Vist per última vegada {time}","instanceCount":{"one":"{count} instància","many":"{count} instàncies","other":"{count} instàncies"},"forms":{"container":"Contenidor","plugin":"Plugin d'Unraid","desktop":"App d'escriptori","android":"App d'Android","extension":"Extensió del navegador"},"open":"Obre","remove":"Treu","memberOn":"{app} a {device}","outHint":"Uneix-te a una fortalesa i aquí veuràs les altres instàncies de {app}, i també les altres apps si vols.","removedHereTitle":"Tret de la fortalesa","removedHereHint":"Una altra app de la fortalesa ha tret {app}. Introdueix la frase actual per tornar a entrar.","enterPhrase":"Introdueix la frase","joinTitle":"Uneix-te a una fortalesa","word":"Paraula {n}","phraseWhere":"Totes les apps de la fortalesa mostren la frase a Instàncies. Enganxa-la al primer camp i es reparteix per la resta.","join":"Uneix-te","joinRefused":"Aquestes paraules no obren cap fortalesa. Revisa-les i torna-ho a provar.","joinedTitle":"{app} ja és a la fortalesa","cancel":"Cancel·la","close":"Tanca","leaveTitle":"Vols sortir de la fortalesa?","leaveBody":"Les altres apps ja no veuran {app}, i s'acaben tots els permisos amb {app}.","leaveBack":"Pots tornar amb la frase actual.","leaveConfirm":"Surt","removeTitle":"Vols treure {device}?","removeBody":"{device} perd l'accés de seguida. Això no es pot desfer.","removeBack":"Qui encara tingui la frase hi pot tornar a entrar fins que la canviïs.","removedTitle":"Vols canviar la frase ara?","removedBody":"{device} ja és fora. Mentre la frase sigui la mateixa, el camí de tornada continua obert.","removedNote":"Has tret: {devices}. Canvia la frase per tancar el camí de tornada.","phraseTitle":"Frase","leaveRow":"Aquesta app a la fortalesa","leaveAbout":"Treu {app} de la fortalesa. Amb la frase actual hi pot tornar.","changedTitle":"Frase nova","changeElsewhere":"La frase la canvies a {apps}.","changeNowhere":"{app} no guarda la frase. Canvia-la en una app de la fortalesa que la mostri.","phraseMoved":"La teva fortalesa té una frase nova que {app} no ha pogut seguir tot sol. Introdueix les paraules noves i continuarà sent el mateix membre.","failed":"No ha funcionat. Torna-ho a provar.","statusWhere":"{app} guarda la configuració a Unraid, a {device}.","statusLoading":"Preguntant a {app} com va.","statusQuiet":"Ara mateix res no et necessita i no hi ha res en marxa.","statusNeedsYou":"Et necessita","statusWork":"En marxa ara","statusNext":"Pròximament","statusLog":"Últimes entrades","states":{"off":"Aturat","ok":"Tot bé","starting":"Iniciant","warning":"Cal una ullada","overdue":"Endarrerit","failed":"Ha fallat"},"levels":{"warn":"Avís","error":"Error"}},
    "cs": {"fortress":"Pevnost","showOthers":"Zobrazit ostatní aplikace pevnosti","about":"V pevnosti se tvoje aplikace navzájem vidí a ulehčují si práci. Aplikace se do ní dostane pomocí fráze.","othersApps":{"one":"{count} další aplikace","few":"{count} další aplikace","many":"{count} další aplikace","other":"{count} dalších aplikací"},"othersDevices":{"one":"{count} zařízení","few":"{count} zařízeních","many":"{count} zařízení","other":"{count} zařízeních"},"othersLine":"{apps} na {devices}. Platí jen pro toto zařízení.","connected":"Připojeno","away":"Nedostupné","viaRelay":"Přes přeposílač","lastSeen":"Naposledy viděno {time}","instanceCount":{"one":"{count} instance","few":"{count} instance","many":"{count} instance","other":"{count} instancí"},"forms":{"container":"Kontejner","plugin":"Plugin pro Unraid","desktop":"Desktopová aplikace","android":"Aplikace pro Android","extension":"Rozšíření prohlížeče"},"open":"Otevřít","remove":"Odebrat","memberOn":"{app} na {device}","outHint":"Připoj se k pevnosti a uvidíš tu své další instance aplikace {app}, a pokud chceš, i ostatní aplikace.","removedHereTitle":"Odebráno z pevnosti","removedHereHint":"Jiná aplikace v pevnosti odebrala {app}. Zadej aktuální frázi a vrátíš se zpátky.","enterPhrase":"Zadat frázi","joinTitle":"Připojit se k pevnosti","word":"Slovo {n}","phraseWhere":"Frázi najdeš v každé aplikaci pevnosti v části Instance. Vlož ji do prvního pole a sama se rozdělí do ostatních.","join":"Připojit","joinRefused":"Tato slova neotevírají žádnou pevnost. Zkontroluj je a zkus to znovu.","joinedTitle":"Aplikace {app} je teď v pevnosti","cancel":"Zrušit","close":"Zavřít","leaveTitle":"Opustit pevnost?","leaveBody":"Ostatní aplikace už aplikaci {app} neuvidí a všechna oprávnění aplikace {app} skončí.","leaveBack":"Vrátit se můžeš pomocí aktuální fráze.","leaveConfirm":"Opustit","removeTitle":"Odebrat instanci {device}?","removeBody":"{device} hned přijde o přístup. Nejde to vrátit zpět.","removeBack":"Kdo frázi pořád má, dostane se zpátky, dokud ji nezměníš.","removedTitle":"Změnit frázi hned teď?","removedBody":"{device} je venku. Dokud zůstane fráze stejná, cesta zpátky je ale pořád otevřená.","removedNote":"Odebráno: {devices}. Změň frázi a cesta zpátky se zavře.","phraseTitle":"Fráze","leaveRow":"Tato aplikace v pevnosti","leaveAbout":"Odebere aplikaci {app} z pevnosti. S aktuální frází se může vrátit.","changedTitle":"Nová fráze","changeElsewhere":"Frázi změníš v {apps}.","changeNowhere":"{app} si frázi neuchovává. Změň ji v aplikaci pevnosti, která ji ukazuje.","phraseMoved":"Tvoje pevnost má novou frázi, ke které {app} nedokázal přejít sám. Zadej nová slova a zůstane stejným členem.","failed":"Nepovedlo se. Zkus to prosím znovu.","statusWhere":"Nastavení aplikace {app} najdeš v systému Unraid na zařízení {device}.","statusLoading":"Zjišťuji, jak se daří aplikaci {app}.","statusQuiet":"Teď tě nic nepotřebuje a nic neběží.","statusNeedsYou":"Potřebuje tě","statusWork":"Právě běží","statusNext":"Na řadě","statusLog":"Poslední záznamy","states":{"off":"Vypnuto","ok":"Vše v pořádku","starting":"Spouští se","warning":"Chce to kontrolu","overdue":"Po termínu","failed":"Selhalo"},"levels":{"warn":"Varování","error":"Chyba"}},
    "da": {"fortress":"Fæstning","showOthers":"Vis fæstningens andre apps","about":"I en fæstning kan dine apps se hinanden og dele arbejdet. En app kommer ind med sætningen.","othersApps":{"one":"{count} anden app","other":"{count} andre apps"},"othersDevices":{"one":"{count} enhed","other":"{count} enheder"},"othersLine":"{apps} på {devices}. Gælder kun på denne enhed.","connected":"Forbundet","away":"Ikke tilgængelig","viaRelay":"Via videresender","lastSeen":"Sidst set {time}","instanceCount":{"one":"{count} instans","other":"{count} instanser"},"forms":{"container":"Container","plugin":"Unraid-plugin","desktop":"Desktop-app","android":"Android-app","extension":"Browserudvidelse"},"open":"Åbn","remove":"Fjern","memberOn":"{app} på {device}","outHint":"Tilslut dig en fæstning, så vises dine andre instanser af {app} her og, hvis du vil, også de andre apps.","removedHereTitle":"Fjernet fra fæstningen","removedHereHint":"En anden app i fæstningen har fjernet {app}. Indtast den aktuelle sætning for at komme ind igen.","enterPhrase":"Indtast sætning","joinTitle":"Tilslut dig en fæstning","word":"Ord {n}","phraseWhere":"Alle apps i fæstningen viser sætningen under Instanser. Indsæt den i det første felt, så fordeler den sig over resten.","join":"Tilslut","joinRefused":"Disse ord åbner ingen fæstning. Tjek dem, og prøv igen.","joinedTitle":"{app} er nu i fæstningen","cancel":"Annuller","close":"Luk","leaveTitle":"Forlad fæstningen?","leaveBody":"De andre apps kan ikke længere se {app}, og alle tilladelser med {app} ophører.","leaveBack":"Du kan komme tilbage med den aktuelle sætning.","leaveConfirm":"Forlad","removeTitle":"Fjern {device}?","removeBody":"{device} mister sin adgang med det samme. Det kan ikke fortrydes.","removeBack":"Alle, der stadig har sætningen, kan komme ind igen, indtil du skifter den.","removedTitle":"Skift sætningen nu?","removedBody":"{device} er ude. Så længe sætningen er den samme, står vejen tilbage dog stadig åben.","removedNote":"Fjernet: {devices}. Skift sætningen for at lukke vejen tilbage.","phraseTitle":"Sætning","leaveRow":"Denne app i fæstningen","leaveAbout":"Tager {app} ud af fæstningen. Med den aktuelle sætning kommer den tilbage.","changedTitle":"Ny sætning","changeElsewhere":"Sætningen skifter du i {apps}.","changeNowhere":"{app} gemmer ikke sætningen. Skift den i en app i fæstningen, der viser den.","phraseMoved":"Din fæstning har en ny sætning, som {app} ikke kunne følge af sig selv. Indtast de nye ord, så fortsætter den som det samme medlem.","failed":"Det lykkedes ikke. Prøv igen.","statusWhere":"{app} har sine indstillinger i Unraid på {device}.","statusLoading":"Spørger {app}, hvordan det går.","statusQuiet":"Intet venter på dig lige nu, og intet kører.","statusNeedsYou":"Venter på dig","statusWork":"Kører nu","statusNext":"Næste","statusLog":"Seneste poster","states":{"off":"Fra","ok":"Alt i orden","starting":"Starter","warning":"Bør ses på","overdue":"Forsinket","failed":"Mislykket"},"levels":{"warn":"Advarsel","error":"Fejl"}},
    "el": {"fortress":"Φρούριο","showOthers":"Εμφάνιση των άλλων εφαρμογών του φρουρίου","about":"Σε ένα φρούριο οι εφαρμογές σας βλέπουν η μία την άλλη και μοιράζονται τη δουλειά. Μια εφαρμογή μπαίνει με τη φράση.","othersApps":{"one":"{count} ακόμη εφαρμογή","other":"{count} ακόμη εφαρμογές"},"othersDevices":{"one":"{count} συσκευή","other":"{count} συσκευές"},"othersLine":"{apps} σε {devices}. Ισχύει μόνο για αυτή τη συσκευή.","connected":"Σε σύνδεση","away":"Μη προσβάσιμο","viaRelay":"Μέσω αναμεταδότη","lastSeen":"Τελευταία εμφάνιση {time}","instanceCount":{"one":"{count} εγκατάσταση","other":"{count} εγκαταστάσεις"},"forms":{"container":"Κοντέινερ","plugin":"Πρόσθετο Unraid","desktop":"Εφαρμογή υπολογιστή","android":"Εφαρμογή Android","extension":"Επέκταση προγράμματος περιήγησης"},"open":"Άνοιγμα","remove":"Αφαίρεση","memberOn":"{app} στο {device}","outHint":"Μπείτε σε ένα φρούριο και εδώ θα εμφανίζονται οι άλλες εγκαταστάσεις σας της εφαρμογής {app}, και αν θέλετε, οι άλλες εφαρμογές.","removedHereTitle":"Αφαιρέθηκε από το φρούριο","removedHereHint":"Μια άλλη εφαρμογή του φρουρίου αφαίρεσε το {app}. Εισαγάγετε την τρέχουσα φράση για να ξαναμπείτε.","enterPhrase":"Εισαγωγή φράσης","joinTitle":"Ένταξη σε φρούριο","word":"Λέξη {n}","phraseWhere":"Κάθε εφαρμογή του φρουρίου δείχνει τη φράση στις Εγκαταστάσεις. Επικολλήστε την στο πρώτο πεδίο και θα μοιραστεί στα υπόλοιπα.","join":"Ένταξη","joinRefused":"Αυτές οι λέξεις δεν ανοίγουν κανένα φρούριο. Ελέγξτε τις και δοκιμάστε ξανά.","joinedTitle":"Η εφαρμογή {app} είναι πλέον στο φρούριο","cancel":"Ακύρωση","close":"Κλείσιμο","leaveTitle":"Αποχώρηση από το φρούριο","leaveBody":"Οι άλλες εφαρμογές δεν θα βλέπουν πια την εφαρμογή {app}, και κάθε άδεια με την εφαρμογή {app} λήγει.","leaveBack":"Μπορείτε να επιστρέψετε με την τρέχουσα φράση.","leaveConfirm":"Αποχώρηση","removeTitle":"Αφαίρεση της εγκατάστασης {device}","removeBody":"Το {device} χάνει αμέσως την πρόσβασή του. Αυτό δεν αναιρείται.","removeBack":"Όποιος έχει ακόμη τη φράση μπορεί να ξαναμπεί, μέχρι να την αλλάξετε.","removedTitle":"Αλλαγή της φράσης τώρα;","removedBody":"Το {device} είναι πια έξω. Όσο η φράση μένει ίδια, ο δρόμος της επιστροφής παραμένει ανοιχτός.","removedNote":"Αφαιρέθηκαν: {devices}. Αλλάξτε τη φράση για να κλείσει ο δρόμος της επιστροφής.","phraseTitle":"Φράση","leaveRow":"Αυτή η εφαρμογή στο φρούριο","leaveAbout":"Βγάζει την εφαρμογή {app} από το φρούριο. Με την τρέχουσα φράση επιστρέφει.","changedTitle":"Νέα φράση","changeElsewhere":"Τη φράση την αλλάζετε στο {apps}.","changeNowhere":"Το {app} δεν κρατά τη φράση. Αλλάξτε την σε μια εφαρμογή του φρουρίου που τη δείχνει.","phraseMoved":"Το φρούριό σας έχει νέα φράση, την οποία το {app} δεν μπόρεσε να ακολουθήσει μόνο του. Εισαγάγετε τις νέες λέξεις και θα συνεχίσει ως το ίδιο μέλος.","failed":"Κάτι δεν πήγε καλά. Δοκιμάστε ξανά.","statusWhere":"Οι ρυθμίσεις του {app} βρίσκονται στο Unraid της συσκευής {device}.","statusLoading":"Ρωτάμε το {app} πώς πάει.","statusQuiet":"Τίποτα δεν σας χρειάζεται τώρα και τίποτα δεν εκτελείται.","statusNeedsYou":"Σας χρειάζεται","statusWork":"Εκτελείται τώρα","statusNext":"Ακολουθεί","statusLog":"Τελευταίες εγγραφές","states":{"off":"Ανενεργό","ok":"Όλα καλά","starting":"Ξεκινά","warning":"Θέλει μια ματιά","overdue":"Εκπρόθεσμο","failed":"Απέτυχε"},"levels":{"warn":"Προειδοποίηση","error":"Σφάλμα"}},
    "es": {"fortress":"Fortaleza","showOthers":"Mostrar las otras apps de la fortaleza","about":"En una fortaleza, tus apps se ven entre sí y se reparten el trabajo. Una app entra con la frase.","othersApps":{"one":"{count} app más","many":"{count} apps más","other":"{count} apps más"},"othersDevices":{"one":"{count} dispositivo","many":"{count} dispositivos","other":"{count} dispositivos"},"othersLine":"{apps} en {devices}. Solo se aplica a este dispositivo.","connected":"Conectado","away":"No accesible","viaRelay":"Vía repetidor","lastSeen":"Visto por última vez {time}","instanceCount":{"one":"{count} instancia","many":"{count} instancias","other":"{count} instancias"},"forms":{"container":"Contenedor","plugin":"Plugin de Unraid","desktop":"App de escritorio","android":"App de Android","extension":"Extensión del navegador"},"open":"Abrir","remove":"Quitar","memberOn":"{app} en {device}","outHint":"Únete a una fortaleza y aquí verás tus otras instancias de {app}, y también las demás apps si quieres.","removedHereTitle":"Fuera de la fortaleza","removedHereHint":"Otra app de la fortaleza ha quitado {app}. Introduce la frase actual para volver a entrar.","enterPhrase":"Introducir frase","joinTitle":"Unirse a una fortaleza","word":"Palabra {n}","phraseWhere":"Todas las apps de la fortaleza muestran la frase en Instancias. Pégala en el primer campo y se reparte por los demás.","join":"Unirse","joinRefused":"Estas palabras no abren ninguna fortaleza. Revísalas e inténtalo de nuevo.","joinedTitle":"{app} ya está en la fortaleza","cancel":"Cancelar","close":"Cerrar","leaveTitle":"¿Salir de la fortaleza?","leaveBody":"Las demás apps ya no verán {app}, y todos los permisos con {app} terminan.","leaveBack":"Puedes volver con la frase actual.","leaveConfirm":"Salir","removeTitle":"¿Quitar {device}?","removeBody":"{device} pierde el acceso al momento. Esto no se puede deshacer.","removeBack":"Quien todavía tenga la frase puede volver a entrar hasta que la cambies.","removedTitle":"¿Cambiar la frase ahora?","removedBody":"{device} ya está fuera. Mientras la frase siga igual, el camino de vuelta sigue abierto.","removedNote":"Quitados: {devices}. Cambia la frase para cerrar el camino de vuelta.","phraseTitle":"Frase","leaveRow":"Esta app en la fortaleza","leaveAbout":"Saca {app} de la fortaleza. Con la frase actual puede volver.","changedTitle":"Frase nueva","changeElsewhere":"La frase la cambias en {apps}.","changeNowhere":"{app} no guarda la frase. Cámbiala en una app de la fortaleza que la muestre.","phraseMoved":"Tu fortaleza tiene una frase nueva que {app} no pudo seguir por sí solo. Introduce las palabras nuevas y seguirá siendo el mismo miembro.","failed":"No ha funcionado. Inténtalo de nuevo.","statusWhere":"{app} guarda sus ajustes en Unraid, en {device}.","statusLoading":"Preguntando a {app} cómo va.","statusQuiet":"Ahora mismo nada te necesita y no hay nada en marcha.","statusNeedsYou":"Te necesita","statusWork":"En marcha","statusNext":"Próximamente","statusLog":"Últimas entradas","states":{"off":"Apagado","ok":"Todo bien","starting":"Iniciando","warning":"Conviene revisarlo","overdue":"Atrasado","failed":"Ha fallado"},"levels":{"warn":"Aviso","error":"Error"}},
    "et": {"fortress":"Kindlus","showOthers":"Näita kindluse teisi rakendusi","about":"Kindluses näevad su rakendused üksteist ja jagavad omavahel tööd. Rakendus pääseb sisse fraasiga.","othersApps":{"one":"{count} muu rakendus","other":"{count} muud rakendust"},"othersDevices":{"one":"{count} seadmes","other":"{count} seadmes"},"othersLine":"{apps} {devices}. Kehtib ainult selles seadmes.","connected":"Ühendatud","away":"Ei ole kättesaadav","viaRelay":"Edastaja kaudu","lastSeen":"Viimati nähtud {time}","instanceCount":{"one":"{count} eksemplar","other":"{count} eksemplari"},"forms":{"container":"Konteiner","plugin":"Unraid-plugin","desktop":"Töölauarakendus","android":"Android-rakendus","extension":"Brauserilaiendus"},"open":"Ava","remove":"Eemalda","memberOn":"{app} seadmes {device}","outHint":"Liitu kindlusega ja siin on näha rakenduse {app} teised eksemplarid ning soovi korral ka teised rakendused.","removedHereTitle":"Kindlusest eemaldatud","removedHereHint":"Mõni teine kindluse rakendus eemaldas {app}. Sisesta praegune fraas, et tagasi pääseda.","enterPhrase":"Sisesta fraas","joinTitle":"Liitu kindlusega","word":"Sõna {n}","phraseWhere":"Iga kindluse rakendus näitab fraasi jaotises Eksemplarid. Kleebi see esimesse välja ja see jaotub ülejäänud väljadele.","join":"Liitu","joinRefused":"Need sõnad ei ava ühtegi kindlust. Kontrolli neid ja proovi uuesti.","joinedTitle":"{app} on nüüd kindluses","cancel":"Tühista","close":"Sulge","leaveTitle":"Kas lahkud kindlusest?","leaveBody":"Teised rakendused ei näe enam rakendust {app} ja kõik rakenduse {app} load lõpevad.","leaveBack":"Praeguse fraasiga saad tagasi tulla.","leaveConfirm":"Lahku","removeTitle":"Kas eemaldad eksemplari {device}?","removeBody":"{device} kaotab juurdepääsu kohe. Seda ei saa tagasi võtta.","removeBack":"Kellel fraas veel on, see pääseb tagasi, kuni sa selle muudad.","removedTitle":"Kas muudad fraasi kohe?","removedBody":"{device} on väljas. Kuni fraas jääb samaks, on tee tagasi siiski lahti.","removedNote":"Eemaldatud: {devices}. Muuda fraasi, siis on tee tagasi suletud.","phraseTitle":"Fraas","leaveRow":"See rakendus kindluses","leaveAbout":"Viib rakenduse {app} kindlusest välja. Praeguse fraasiga saab see tagasi tulla.","changedTitle":"Uus fraas","changeElsewhere":"Fraasi muudad rakenduses {apps}.","changeNowhere":"{app} ei hoia fraasi alles. Muuda seda kindluse rakenduses, mis seda näitab.","phraseMoved":"Sinu kindlusel on uus fraas, millele {app} ise järele ei jõudnud. Sisesta uued sõnad ja see jääb samaks liikmeks.","failed":"See ei õnnestunud. Proovi uuesti.","statusWhere":"Rakenduse {app} seaded asuvad seadme {device} Unraid-liideses.","statusLoading":"Küsin rakenduselt {app}, kuidas tal läheb.","statusQuiet":"Praegu ei vaja sind miski ja midagi ei tööta.","statusNeedsYou":"Vajab sind","statusWork":"Praegu käimas","statusNext":"Tulekul","statusLog":"Viimased kirjed","states":{"off":"Väljas","ok":"Kõik korras","starting":"Käivitub","warning":"Vajab pilku","overdue":"Hilinenud","failed":"Ebaõnnestus"},"levels":{"warn":"Hoiatus","error":"Viga"}},
    "eu": {"fortress":"Gotorlekua","showOthers":"Erakutsi gotorlekuko beste aplikazioak","about":"Gotorleku batean zure aplikazioek elkar ikusten dute eta elkarri lana arintzen diote. Aplikazio bat esaldiarekin sartzen da.","othersApps":{"one":"Beste {count} aplikazio","other":"Beste {count} aplikazio"},"othersDevices":{"one":"{count} gailutan","other":"{count} gailutan"},"othersLine":"{apps} {devices}. Gailu honetan bakarrik balio du.","connected":"Konektatuta","away":"Ez dago eskuragarri","viaRelay":"Bitartekari bidez","lastSeen":"Azken aldiz ikusia: {time}","instanceCount":{"one":"{count} instantzia","other":"{count} instantzia"},"forms":{"container":"Edukiontzia","plugin":"Unraid plugina","desktop":"Mahaigaineko aplikazioa","android":"Android aplikazioa","extension":"Nabigatzailearen luzapena"},"open":"Ireki","remove":"Kendu","memberOn":"{app} ({device})","outHint":"Sartu gotorleku batean, eta {app} aplikazioaren beste instantziak hemen agertuko dira, nahi baduzu beste aplikazioekin batera.","removedHereTitle":"Gotorlekutik kendua","removedHereHint":"Gotorlekuko beste aplikazio batek {app} kendu du. Idatzi uneko esaldia berriro sartzeko.","enterPhrase":"Idatzi esaldia","joinTitle":"Sartu gotorleku batean","word":"{n}. hitza","phraseWhere":"Gotorlekuko aplikazio guztiek Instantziak atalean erakusten dute esaldia. Itsatsi lehen eremuan, eta gainerakoetara banatuko da.","join":"Sartu","joinRefused":"Hitz hauek ez dute gotorlekurik irekitzen. Egiaztatu eta saiatu berriro.","joinedTitle":"{app} aplikazioa gotorlekuan dago orain","cancel":"Utzi","close":"Itxi","leaveTitle":"Gotorlekutik irten nahi duzu?","leaveBody":"Beste aplikazioek ez dute {app} aplikazioa gehiago ikusiko, eta {app} aplikazioarekiko baimen guztiak amaituko dira.","leaveBack":"Uneko esaldiarekin itzul zaitezke.","leaveConfirm":"Irten","removeTitle":"{device} kendu?","removeBody":"{device} sarbiderik gabe geratzen da berehala. Ezin da desegin.","removeBack":"Esaldia oraindik duen edonor itzul daiteke, zuk aldatu arte.","removedTitle":"Esaldia orain aldatu?","removedBody":"{device} kanpoan dago. Esaldia berdina den bitartean, itzulerako bidea irekita dago oraindik.","removedNote":"Kenduak: {devices}. Aldatu esaldia itzulerako bidea ixteko.","phraseTitle":"Esaldia","leaveRow":"Aplikazio hau gotorlekuan","leaveAbout":"{app} gotorlekutik ateratzen du. Uneko esaldiarekin itzul daiteke.","changedTitle":"Esaldi berria","changeElsewhere":"Esaldia hemen aldatzen duzu: {apps}.","changeNowhere":"{app} aplikazioak ez du esaldia gordetzen. Aldatu esaldia erakusten duen gotorlekuko aplikazio batean.","phraseMoved":"Zure gotorlekuak esaldi berria du, eta {app} aplikazioa ezin izan da bere kabuz egokitu. Idatzi hitz berriak eta kide bera izaten jarraituko du.","failed":"Ez du funtzionatu. Saiatu berriro.","statusWhere":"{app} aplikazioak Unraid-en ditu ezarpenak, {device} gailuan.","statusLoading":"{app} aplikazioari nola dagoen galdetzen.","statusQuiet":"Orain ez dago zure zain ezer, eta ez dago ezer martxan.","statusNeedsYou":"Zure zain","statusWork":"Orain martxan","statusNext":"Hurrengoa","statusLog":"Azken sarrerak","states":{"off":"Itzalita","ok":"Dena ondo","starting":"Abiatzen","warning":"Begiratu behar da","overdue":"Atzeratuta","failed":"Huts egin du"},"levels":{"warn":"Abisua","error":"Errorea"}},
    "fa": {"fortress":"دژ","showOthers":"نمایش برنامه‌های دیگر دژ","about":"در دژ برنامه‌های شما یکدیگر را می‌بینند و کار را از دوش هم برمی‌دارند. هر برنامه با عبارت وارد دژ می‌شود.","othersApps":{"one":"{count} برنامهٔ دیگر","other":"{count} برنامهٔ دیگر"},"othersDevices":{"one":"{count} دستگاه","other":"{count} دستگاه"},"othersLine":"{apps} روی {devices}. فقط برای همین دستگاه اعمال می‌شود.","connected":"متصل","away":"در دسترس نیست","viaRelay":"از طریق رله","lastSeen":"آخرین حضور: {time}","instanceCount":{"one":"{count} نمونه","other":"{count} نمونه"},"forms":{"container":"کانتینر","plugin":"افزونهٔ Unraid","desktop":"برنامهٔ دسکتاپ","android":"برنامهٔ Android","extension":"افزونهٔ مرورگر"},"open":"باز کردن","remove":"حذف","memberOn":"{app} روی {device}","outHint":"به یک دژ بپیوندید تا نمونه‌های دیگر {app} اینجا نمایش داده شوند، و اگر بخواهید، برنامه‌های دیگر هم.","removedHereTitle":"از دژ حذف شد","removedHereHint":"برنامهٔ دیگری در دژ، {app} را حذف کرد. برای ورود دوباره، عبارت کنونی را وارد کنید.","enterPhrase":"وارد کردن عبارت","joinTitle":"پیوستن به دژ","word":"واژهٔ {n}","phraseWhere":"هر برنامهٔ داخل دژ عبارت را در بخش «نمونه‌ها» نشان می‌دهد. آن را در خانهٔ اول بچسبانید تا در بقیهٔ خانه‌ها پخش شود.","join":"پیوستن","joinRefused":"این واژه‌ها هیچ دژی را باز نمی‌کنند. آن‌ها را بررسی کنید و دوباره امتحان کنید.","joinedTitle":"{app} حالا در دژ است","cancel":"لغو","close":"بستن","leaveTitle":"از دژ خارج می‌شوید؟","leaveBody":"از این پس برنامه‌های دیگر {app} را نمی‌بینند و هر اجازه‌ای که با {app} دارند پایان می‌یابد.","leaveBack":"با عبارت کنونی می‌توانید برگردید.","leaveConfirm":"خروج","removeTitle":"{device} حذف شود؟","removeBody":"{device} بی‌درنگ دسترسی‌اش را از دست می‌دهد. این کار برگشت‌پذیر نیست.","removeBack":"هر کسی که هنوز عبارت را دارد، تا وقتی آن را عوض نکرده‌اید می‌تواند برگردد.","removedTitle":"عبارت همین حالا عوض شود؟","removedBody":"{device} بیرون است. ولی تا وقتی عبارت همان بماند، راه برگشت هنوز باز است.","removedNote":"حذف‌شده: {devices}. عبارت را عوض کنید تا راه برگشت بسته شود.","phraseTitle":"عبارت","leaveRow":"این برنامه در دژ","leaveAbout":"{app} را از دژ بیرون می‌برد. با عبارت کنونی برمی‌گردد.","changedTitle":"عبارت تازه","changeElsewhere":"عبارت را در {apps} تغییر می‌دهید.","changeNowhere":"{app} عبارت را نگه نمی‌دارد. آن را در برنامه‌ای از دژ تغییر دهید که عبارت را نشان می‌دهد.","phraseMoved":"دژ شما عبارت تازه‌ای دارد که {app} نتوانست خودش با آن همراه شود. واژه‌های تازه را وارد کنید تا همان عضو بماند.","failed":"انجام نشد. لطفاً دوباره امتحان کنید.","statusWhere":"{app} تنظیماتش را در Unraid روی {device} نگه می‌دارد.","statusLoading":"در حال پرسیدن حال {app}.","statusQuiet":"فعلاً چیزی به شما نیاز ندارد و چیزی هم در حال اجرا نیست.","statusNeedsYou":"به شما نیاز دارد","statusWork":"در حال اجرا","statusNext":"در پیش","statusLog":"آخرین ورودی‌ها","states":{"off":"خاموش","ok":"همه‌چیز خوب است","starting":"در حال شروع","warning":"نیاز به بررسی","overdue":"عقب افتاده","failed":"ناموفق"},"levels":{"warn":"هشدار","error":"خطا"}},
    "fi": {"fortress":"Linnoitus","showOthers":"Näytä linnoituksen muut sovellukset","about":"Linnoituksessa sovelluksesi näkevät toisensa ja jakavat töitä keskenään. Sovellus pääsee sisään lauseella.","othersApps":{"one":"{count} muu sovellus","other":"{count} muuta sovellusta"},"othersDevices":{"one":"{count} laitteella","other":"{count} laitteella"},"othersLine":"{apps} {devices}. Koskee vain tätä laitetta.","connected":"Yhdistetty","away":"Ei tavoitettavissa","viaRelay":"Releen kautta","lastSeen":"Nähty viimeksi {time}","instanceCount":{"one":"{count} instanssi","other":"{count} instanssia"},"forms":{"container":"Kontti","plugin":"Unraid-laajennus","desktop":"Työpöytäsovellus","android":"Android-sovellus","extension":"Selainlaajennus"},"open":"Avaa","remove":"Poista","memberOn":"{app} laitteella {device}","outHint":"Liity linnoitukseen, niin sovelluksen {app} muut instanssit näkyvät tässä, ja halutessasi myös muut sovellukset.","removedHereTitle":"Poistettu linnoituksesta","removedHereHint":"Toinen linnoituksen sovellus poisti sovelluksen {app}. Anna nykyinen lause, niin pääset takaisin.","enterPhrase":"Syötä lause","joinTitle":"Liity linnoitukseen","word":"Sana {n}","phraseWhere":"Jokainen linnoituksen sovellus näyttää lauseen kohdassa Instanssit. Liitä se ensimmäiseen kenttään, niin se jakautuu muihin kenttiin.","join":"Liity","joinRefused":"Nämä sanat eivät avaa mitään linnoitusta. Tarkista ne ja yritä uudelleen.","joinedTitle":"{app} on nyt linnoituksessa","cancel":"Peruuta","close":"Sulje","leaveTitle":"Poistutaanko linnoituksesta?","leaveBody":"Muut sovellukset eivät enää näe sovellusta {app}, ja kaikki sovelluksen {app} luvat päättyvät.","leaveBack":"Voit palata nykyisellä lauseella.","leaveConfirm":"Poistu","removeTitle":"Poistetaanko instanssi {device}?","removeBody":"{device} menettää pääsynsä heti. Tätä ei voi perua.","removeBack":"Kuka tahansa, jolla lause vielä on, pääsee takaisin, kunnes vaihdat sen.","removedTitle":"Vaihdetaanko lause nyt?","removedBody":"{device} on ulkona. Niin kauan kuin lause pysyy samana, tie takaisin on silti auki.","removedNote":"Poistettu: {devices}. Vaihda lause, niin tie takaisin sulkeutuu.","phraseTitle":"Lause","leaveRow":"Tämä sovellus linnoituksessa","leaveAbout":"Vie sovelluksen {app} pois linnoituksesta. Nykyisellä lauseella se pääsee takaisin.","changedTitle":"Uusi lause","changeElsewhere":"Lauseen vaihdat sovelluksessa {apps}.","changeNowhere":"{app} ei säilytä lausetta. Vaihda se linnoituksen sovelluksessa, joka näyttää sen.","phraseMoved":"Linnoituksellasi on uusi lause, jota {app} ei pystynyt seuraamaan itse. Anna uudet sanat, niin se jatkaa samana jäsenenä.","failed":"Se ei onnistunut. Yritä uudelleen.","statusWhere":"Sovelluksen {app} asetukset löytyvät laitteen {device} Unraid-käyttöliittymästä.","statusLoading":"Kysytään sovellukselta {app}, mitä kuuluu.","statusQuiet":"Mikään ei tarvitse sinua juuri nyt, eikä mitään ole käynnissä.","statusNeedsYou":"Tarvitsee sinua","statusWork":"Käynnissä nyt","statusNext":"Seuraavaksi","statusLog":"Viimeisimmät merkinnät","states":{"off":"Pois","ok":"Kaikki hyvin","starting":"Käynnistyy","warning":"Kaipaa vilkaisua","overdue":"Myöhässä","failed":"Epäonnistui"},"levels":{"warn":"Varoitus","error":"Virhe"}},
    "fr": {"fortress":"Forteresse","showOthers":"Afficher les autres apps de la forteresse","about":"Dans une forteresse, vos apps se voient et se partagent le travail. Une app y entre avec la phrase.","othersApps":{"one":"{count} autre app","many":"{count} autres apps","other":"{count} autres apps"},"othersDevices":{"one":"{count} appareil","many":"{count} appareils","other":"{count} appareils"},"othersLine":"{apps} sur {devices}. Ne s'applique qu'à cet appareil.","connected":"Connecté","away":"Injoignable","viaRelay":"Via relais","lastSeen":"Vu pour la dernière fois {time}","instanceCount":{"one":"{count} instance","many":"{count} instances","other":"{count} instances"},"forms":{"container":"Conteneur","plugin":"Plugin Unraid","desktop":"App de bureau","android":"App Android","extension":"Extension de navigateur"},"open":"Ouvrir","remove":"Retirer","memberOn":"{app} sur {device}","outHint":"Rejoignez une forteresse pour retrouver ici vos autres instances de {app}, et les autres apps si vous le souhaitez.","removedHereTitle":"Retiré de la forteresse","removedHereHint":"Une autre app de la forteresse a retiré {app}. Saisissez la phrase actuelle pour revenir.","enterPhrase":"Saisir la phrase","joinTitle":"Rejoindre une forteresse","word":"Mot {n}","phraseWhere":"Chaque app de la forteresse affiche la phrase sous Instances. Collez-la dans le premier champ, elle se répartit sur les autres.","join":"Rejoindre","joinRefused":"Ces mots n'ouvrent aucune forteresse. Vérifiez-les et réessayez.","joinedTitle":"{app} est maintenant dans la forteresse","cancel":"Annuler","close":"Fermer","leaveTitle":"Quitter la forteresse ?","leaveBody":"Les autres apps ne verront plus {app}, et toutes les autorisations avec {app} prennent fin.","leaveBack":"Vous pouvez revenir avec la phrase actuelle.","leaveConfirm":"Quitter","removeTitle":"Retirer {device} ?","removeBody":"{device} perd son accès immédiatement. C'est irréversible.","removeBack":"Toute personne qui a encore la phrase peut revenir tant que vous ne l'avez pas changée.","removedTitle":"Changer la phrase maintenant ?","removedBody":"{device} est dehors. Tant que la phrase reste la même, le chemin du retour reste ouvert.","removedNote":"Retirés : {devices}. Changez la phrase pour fermer le chemin du retour.","phraseTitle":"Phrase","leaveRow":"Cette app dans la forteresse","leaveAbout":"Sort {app} de la forteresse. Avec la phrase actuelle, elle peut revenir.","changedTitle":"Nouvelle phrase","changeElsewhere":"Vous changez la phrase dans {apps}.","changeNowhere":"{app} ne garde pas la phrase. Changez-la dans une app de la forteresse qui l'affiche.","phraseMoved":"Votre forteresse a une nouvelle phrase que {app} n'a pas pu suivre tout seul. Saisissez les nouveaux mots et il reste le même membre.","failed":"Cela n'a pas fonctionné. Veuillez réessayer.","statusWhere":"{app} garde ses réglages dans Unraid sur {device}.","statusLoading":"On demande des nouvelles à {app}.","statusQuiet":"Rien ne vous attend pour l'instant, et rien n'est en cours.","statusNeedsYou":"Vous attend","statusWork":"En cours","statusNext":"À venir","statusLog":"Dernières entrées","states":{"off":"Arrêté","ok":"Tout va bien","starting":"Démarrage","warning":"À vérifier","overdue":"En retard","failed":"Échec"},"levels":{"warn":"Avertissement","error":"Erreur"}},
    "gl": {"fortress":"Fortaleza","showOthers":"Amosar as outras apps da fortaleza","about":"Nunha fortaleza, as túas apps vense entre si e repártense o traballo. Unha app entra coa frase.","othersApps":{"one":"{count} app máis","other":"{count} apps máis"},"othersDevices":{"one":"{count} dispositivo","other":"{count} dispositivos"},"othersLine":"{apps} en {devices}. Só se aplica a este dispositivo.","connected":"Conectado","away":"Non accesible","viaRelay":"Por relé","lastSeen":"Visto por última vez {time}","instanceCount":{"one":"{count} instancia","other":"{count} instancias"},"forms":{"container":"Contedor","plugin":"Plugin de Unraid","desktop":"App de escritorio","android":"App de Android","extension":"Extensión do navegador"},"open":"Abrir","remove":"Quitar","memberOn":"{app} en {device}","outHint":"Únete a unha fortaleza e aquí verás as túas outras instancias de {app}, e tamén as demais apps se queres.","removedHereTitle":"Fóra da fortaleza","removedHereHint":"Outra app da fortaleza quitou {app}. Introduce a frase actual para volver entrar.","enterPhrase":"Introducir a frase","joinTitle":"Unirse a unha fortaleza","word":"Palabra {n}","phraseWhere":"Todas as apps da fortaleza amosan a frase en Instancias. Pégaa no primeiro campo e repártese polos demais.","join":"Unirse","joinRefused":"Estas palabras non abren ningunha fortaleza. Revísaas e téntao de novo.","joinedTitle":"{app} xa está na fortaleza","cancel":"Cancelar","close":"Pechar","leaveTitle":"Saír da fortaleza?","leaveBody":"As outras apps xa non verán {app}, e rematan todos os permisos con {app}.","leaveBack":"Podes volver coa frase actual.","leaveConfirm":"Saír","removeTitle":"Quitar {device}?","removeBody":"{device} perde o acceso de inmediato. Isto non se pode desfacer.","removeBack":"Quen aínda teña a frase pode volver entrar ata que a cambies.","removedTitle":"Cambiar a frase agora?","removedBody":"{device} xa está fóra. Mentres a frase siga igual, o camiño de volta segue aberto.","removedNote":"Quitados: {devices}. Cambia a frase para pechar o camiño de volta.","phraseTitle":"Frase","leaveRow":"Esta app na fortaleza","leaveAbout":"Saca {app} da fortaleza. Coa frase actual pode volver.","changedTitle":"Frase nova","changeElsewhere":"A frase cámbiala en {apps}.","changeNowhere":"{app} non garda a frase. Cámbiaa nunha app da fortaleza que a amose.","phraseMoved":"A túa fortaleza ten unha frase nova que {app} non puido seguir por si só. Introduce as palabras novas e seguirá sendo o mesmo membro.","failed":"Non funcionou. Téntao de novo.","statusWhere":"{app} garda a súa configuración en Unraid, en {device}.","statusLoading":"Preguntándolle a {app} que tal vai.","statusQuiet":"Agora mesmo nada te necesita e non hai nada en marcha.","statusNeedsYou":"Necesítate","statusWork":"En marcha","statusNext":"Próximamente","statusLog":"Últimas entradas","states":{"off":"Apagado","ok":"Todo ben","starting":"Iniciando","warning":"Convén revisalo","overdue":"Atrasado","failed":"Fallou"},"levels":{"warn":"Aviso","error":"Erro"}},
    "he": {"fortress":"מבצר","showOthers":"הצגת שאר האפליקציות במבצר","about":"במבצר האפליקציות שלך רואות זו את זו ומורידות זו מזו עבודה. אפליקציה נכנסת עם הביטוי.","othersApps":{"one":"עוד {count} אפליקציה","two":"עוד {count} אפליקציות","other":"עוד {count} אפליקציות"},"othersDevices":{"one":"{count} מכשיר","two":"{count} מכשירים","other":"{count} מכשירים"},"othersLine":"{apps} ב-{devices}. חל רק על המכשיר הזה.","connected":"מחובר","away":"לא נגיש","viaRelay":"דרך ממסר","lastSeen":"נראה לאחרונה {time}","instanceCount":{"one":"{count} מופע","two":"{count} מופעים","other":"{count} מופעים"},"forms":{"container":"קונטיינר","plugin":"תוסף Unraid","desktop":"אפליקציית מחשב","android":"אפליקציית Android","extension":"תוסף דפדפן"},"open":"פתיחה","remove":"הסרה","memberOn":"{app} ב-{device}","outHint":"אחרי הצטרפות למבצר יופיעו כאן המופעים האחרים של {app}, ואם רוצים, גם שאר האפליקציות.","removedHereTitle":"הוסר מהמבצר","removedHereHint":"אפליקציה אחרת במבצר הסירה את {app}. כדי לחזור, צריך להזין את הביטוי הנוכחי.","enterPhrase":"הזנת הביטוי","joinTitle":"הצטרפות למבצר","word":"מילה {n}","phraseWhere":"כל אפליקציה במבצר מציגה את הביטוי תחת ״מופעים״. מדביקים אותו בשדה הראשון והוא מתפזר על השאר.","join":"הצטרפות","joinRefused":"המילים האלה לא פותחות אף מבצר. כדאי לבדוק אותן ולנסות שוב.","joinedTitle":"{app} נמצא עכשיו במבצר","cancel":"ביטול","close":"סגירה","leaveTitle":"לצאת מהמבצר?","leaveBody":"שאר האפליקציות לא יראו יותר את {app}, וכל הרשאה עם {app} מסתיימת.","leaveBack":"אפשר לחזור עם הביטוי הנוכחי.","leaveConfirm":"יציאה","removeTitle":"להסיר את {device}?","removeBody":"{device} מאבד את הגישה מיד. אי אפשר לבטל את זה.","removeBack":"מי שעדיין יש לו את הביטוי יכול לחזור, עד שהביטוי ישתנה.","removedTitle":"לשנות את הביטוי עכשיו?","removedBody":"{device} בחוץ. כל עוד הביטוי נשאר אותו דבר, הדרך חזרה עדיין פתוחה.","removedNote":"הוסרו: {devices}. כדי לסגור את הדרך חזרה, צריך לשנות את הביטוי.","phraseTitle":"ביטוי","leaveRow":"האפליקציה הזאת במבצר","leaveAbout":"מוציא את {app} מהמבצר. עם הביטוי הנוכחי אפשר לחזור.","changedTitle":"ביטוי חדש","changeElsewhere":"את הביטוי משנים ב-{apps}.","changeNowhere":"{app} לא שומר את הביטוי. אפשר לשנות אותו באפליקציה במבצר שמציגה אותו.","phraseMoved":"למבצר יש ביטוי חדש ש-{app} לא הצליח לעבור אליו בעצמו. צריך להזין את המילים החדשות, והוא ימשיך כאותו חבר.","failed":"זה לא הצליח. כדאי לנסות שוב.","statusWhere":"ההגדרות של {app} נמצאות ב-Unraid שעל {device}.","statusLoading":"בודקים מה שלום {app}.","statusQuiet":"כרגע אין שום דבר שמחכה לטיפול, ושום דבר לא רץ.","statusNeedsYou":"מחכה לטיפול","statusWork":"רץ עכשיו","statusNext":"בקרוב","statusLog":"רשומות אחרונות","states":{"off":"כבוי","ok":"הכול תקין","starting":"מתחיל","warning":"כדאי להציץ","overdue":"באיחור","failed":"נכשל"},"levels":{"warn":"אזהרה","error":"שגיאה"}},
    "hi": {"fortress":"किला","showOthers":"किले के दूसरे ऐप दिखाएँ","about":"किले में आपके ऐप एक-दूसरे को देखते हैं और एक-दूसरे का काम बाँट लेते हैं। कोई ऐप वाक्यांश से अंदर आता है।","othersApps":{"one":"{count} और ऐप","other":"{count} और ऐप"},"othersDevices":{"one":"{count} डिवाइस","other":"{count} डिवाइस"},"othersLine":"{devices} पर {apps}। यह सिर्फ़ इस डिवाइस पर लागू होता है।","connected":"कनेक्टेड","away":"पहुँच से बाहर","viaRelay":"रिले के ज़रिए","lastSeen":"आख़िरी बार देखा गया {time}","instanceCount":{"one":"{count} इंस्टेंस","other":"{count} इंस्टेंस"},"forms":{"container":"कंटेनर","plugin":"Unraid प्लगइन","desktop":"डेस्कटॉप ऐप","android":"Android ऐप","extension":"ब्राउज़र एक्सटेंशन"},"open":"खोलें","remove":"हटाएँ","memberOn":"{device} पर {app}","outHint":"किसी किले से जुड़ें, फिर {app} के आपके दूसरे इंस्टेंस यहाँ दिखेंगे, और चाहें तो बाकी ऐप भी।","removedHereTitle":"किले से हटाया गया","removedHereHint":"किले के किसी दूसरे ऐप ने {app} को हटा दिया है। वापस आने के लिए मौजूदा वाक्यांश डालें।","enterPhrase":"वाक्यांश डालें","joinTitle":"किसी किले से जुड़ें","word":"शब्द {n}","phraseWhere":"किले का हर ऐप ‘इंस्टेंस’ में वाक्यांश दिखाता है। इसे पहली फ़ील्ड में चिपकाएँ, बाकी फ़ील्ड में यह अपने-आप भर जाएगा।","join":"जुड़ें","joinRefused":"इन शब्दों से कोई किला नहीं खुलता। इन्हें जाँचें और फिर से कोशिश करें।","joinedTitle":"{app} अब किले में है","cancel":"रद्द करें","close":"बंद करें","leaveTitle":"किला छोड़ें?","leaveBody":"दूसरे ऐप अब {app} को नहीं देखेंगे, और {app} के साथ हर अनुमति खत्म हो जाएगी।","leaveBack":"मौजूदा वाक्यांश से आप वापस आ सकते हैं।","leaveConfirm":"छोड़ें","removeTitle":"{device} हटाएँ?","removeBody":"{device} की पहुँच तुरंत खत्म हो जाएगी। इसे वापस नहीं किया जा सकता।","removeBack":"जब तक आप वाक्यांश नहीं बदलते, जिसके पास वह अब भी है वह वापस आ सकता है।","removedTitle":"वाक्यांश अभी बदलें?","removedBody":"{device} अब बाहर है। लेकिन जब तक वाक्यांश वही रहेगा, वापसी का रास्ता खुला रहेगा।","removedNote":"हटाए गए: {devices}। वापसी का रास्ता बंद करने के लिए वाक्यांश बदलें।","phraseTitle":"वाक्यांश","leaveRow":"किले में यह ऐप","leaveAbout":"{app} को किले से बाहर कर देता है। मौजूदा वाक्यांश से यह वापस आ सकता है।","changedTitle":"नया वाक्यांश","changeElsewhere":"वाक्यांश आप {apps} में बदलते हैं।","changeNowhere":"{app} वाक्यांश नहीं रखता। इसे किले के उस ऐप में बदलें जो इसे दिखाता है।","phraseMoved":"आपके किले का नया वाक्यांश है, जिसे {app} अपने आप नहीं अपना पाया। नए शब्द डालें, तो यह वही सदस्य बना रहेगा।","failed":"यह नहीं हो पाया। कृपया फिर से कोशिश करें।","statusWhere":"{app} की सेटिंग्स {device} पर Unraid में रहती हैं।","statusLoading":"{app} से उसका हाल पूछ रहे हैं।","statusQuiet":"अभी किसी चीज़ को आपकी ज़रूरत नहीं है, और कुछ भी नहीं चल रहा।","statusNeedsYou":"आपकी ज़रूरत है","statusWork":"अभी चल रहा है","statusNext":"आगे आने वाला","statusLog":"हाल की एंट्री","states":{"off":"बंद","ok":"सब ठीक है","starting":"शुरू हो रहा है","warning":"एक नज़र डालें","overdue":"समय से पीछे","failed":"विफल"},"levels":{"warn":"चेतावनी","error":"त्रुटि"}},
    "hr": {"fortress":"Utvrda","showOthers":"Prikaži ostale aplikacije utvrde","about":"U utvrdi se tvoje aplikacije međusobno vide i pomažu si u radu. Aplikacija ulazi u nju s frazom.","othersApps":{"one":"{count} dodatna aplikacija","few":"{count} dodatne aplikacije","other":"{count} dodatnih aplikacija"},"othersDevices":{"one":"{count} uređaju","few":"{count} uređaja","other":"{count} uređaja"},"othersLine":"{apps} na {devices}. Vrijedi samo za ovaj uređaj.","connected":"Povezano","away":"Nedostupno","viaRelay":"Preko releja","lastSeen":"Zadnji put viđeno {time}","instanceCount":{"one":"{count} instanca","few":"{count} instance","other":"{count} instanci"},"forms":{"container":"Kontejner","plugin":"Unraid dodatak","desktop":"Aplikacija za računalo","android":"Aplikacija za Android","extension":"Proširenje preglednika"},"open":"Otvori","remove":"Ukloni","memberOn":"{app} na {device}","outHint":"Pridruži se utvrdi i ovdje će se prikazati tvoje ostale instance aplikacije {app}, a po želji i ostale aplikacije.","removedHereTitle":"Uklonjeno iz utvrde","removedHereHint":"Druga aplikacija u utvrdi uklonila je {app}. Unesi trenutačnu frazu da se vratiš.","enterPhrase":"Unesi frazu","joinTitle":"Pridruži se utvrdi","word":"Riječ {n}","phraseWhere":"Svaka aplikacija u utvrdi prikazuje frazu pod Instance. Zalijepi je u prvo polje i sama će se rasporediti po ostalima.","join":"Pridruži se","joinRefused":"Ove riječi ne otvaraju nijednu utvrdu. Provjeri ih i pokušaj ponovno.","joinedTitle":"Aplikacija {app} sada je u utvrdi","cancel":"Odustani","close":"Zatvori","leaveTitle":"Napustiti utvrdu?","leaveBody":"Ostale aplikacije više neće vidjeti aplikaciju {app}, a sva dopuštenja aplikacije {app} prestat će vrijediti.","leaveBack":"Možeš se vratiti s trenutačnom frazom.","leaveConfirm":"Napusti","removeTitle":"Ukloniti instancu {device}?","removeBody":"{device} odmah gubi pristup. To se ne može poništiti.","removeBack":"Tko još ima frazu, može se vratiti dok je ne promijeniš.","removedTitle":"Promijeniti frazu odmah?","removedBody":"{device} je vani. Dok fraza ostaje ista, put natrag je i dalje otvoren.","removedNote":"Uklonjeno: {devices}. Promijeni frazu da zatvoriš put natrag.","phraseTitle":"Fraza","leaveRow":"Ova aplikacija u utvrdi","leaveAbout":"Vadi aplikaciju {app} iz utvrde. Uz trenutačnu frazu može se vratiti.","changedTitle":"Nova fraza","changeElsewhere":"Frazu mijenjaš u {apps}.","changeNowhere":"{app} ne čuva frazu. Promijeni je u aplikaciji utvrde koja je prikazuje.","phraseMoved":"Tvoja utvrda ima novu frazu koju {app} nije mogao sam slijediti. Unesi nove riječi i ostat će isti član.","failed":"Nije uspjelo. Pokušaj ponovno.","statusWhere":"{app} drži postavke u sustavu Unraid na uređaju {device}.","statusLoading":"Provjeravam kako je {app}.","statusQuiet":"Trenutačno te ništa ne treba i ništa se ne izvodi.","statusNeedsYou":"Treba te","statusWork":"Upravo radi","statusNext":"Slijedi","statusLog":"Zadnji zapisi","states":{"off":"Isključeno","ok":"Sve u redu","starting":"Pokreće se","warning":"Treba pogledati","overdue":"Kasni","failed":"Nije uspjelo"},"levels":{"warn":"Upozorenje","error":"Greška"}},
    "hu": {"fortress":"Erőd","showOthers":"Az erőd többi alkalmazásának megjelenítése","about":"Egy erődben az alkalmazásaid látják egymást, és leveszik egymás válláról a munkát. Egy alkalmazás a jelmondattal jut be.","othersApps":{"one":"Még {count} alkalmazás","other":"Még {count} alkalmazás"},"othersDevices":{"one":"{count} eszközön","other":"{count} eszközön"},"othersLine":"{apps} {devices}. Csak erre az eszközre érvényes.","connected":"Csatlakozva","away":"Nem érhető el","viaRelay":"Relayen keresztül","lastSeen":"Utoljára látva: {time}","instanceCount":{"one":"{count} példány","other":"{count} példány"},"forms":{"container":"Konténer","plugin":"Unraid-bővítmény","desktop":"Asztali alkalmazás","android":"Android-alkalmazás","extension":"Böngészőbővítmény"},"open":"Megnyitás","remove":"Eltávolítás","memberOn":"{app} itt: {device}","outHint":"Lépj be egy erődbe, és itt megjelenik a többi {app}-példányod, ha szeretnéd, a többi alkalmazással együtt.","removedHereTitle":"Eltávolítva az erődből","removedHereHint":"Az erőd egy másik appja eltávolította ezt: {app}. Add meg az aktuális jelmondatot, és visszajutsz.","enterPhrase":"Jelmondat megadása","joinTitle":"Belépés egy erődbe","word":"{n}. szó","phraseWhere":"Az erőd minden alkalmazása a Példányok alatt mutatja a jelmondatot. Illeszd be az első mezőbe, és magától szétoszlik a többire.","join":"Belépés","joinRefused":"Ezek a szavak egy erődöt sem nyitnak. Nézd át őket, és próbáld újra.","joinedTitle":"{app} bent van az erődben","cancel":"Mégse","close":"Bezárás","leaveTitle":"Kilépsz az erődből?","leaveBody":"{app} eltűnik a többi alkalmazás elől, és minden {app}-engedély megszűnik.","leaveBack":"Az aktuális jelmondattal visszajöhetsz.","leaveConfirm":"Kilépés","removeTitle":"Eltávolítod: {device}?","removeBody":"{device} azonnal elveszíti a hozzáférését. Ez nem vonható vissza.","removeBack":"Akinél még megvan a jelmondat, visszajuthat, amíg meg nem változtatod.","removedTitle":"Megváltoztatod most a jelmondatot?","removedBody":"{device} kint van. Amíg a jelmondat ugyanaz marad, a visszaút még nyitva áll.","removedNote":"Eltávolítva: {devices}. Változtasd meg a jelmondatot, és a visszaút bezárul.","phraseTitle":"Jelmondat","leaveRow":"Ez az alkalmazás az erődben","leaveAbout":"{app} kikerül az erődből. Az aktuális jelmondattal visszajöhet.","changedTitle":"Új jelmondat","changeElsewhere":"A jelmondatot itt módosítod: {apps}.","changeNowhere":"A(z) {app} nem őrzi a jelmondatot. Módosítsd az erőd egyik appjában, amely mutatja.","phraseMoved":"Az erődödnek új jelmondata van, amelyet a(z) {app} nem tudott magától követni. Add meg az új szavakat, és ugyanaz a tag marad.","failed":"Ez nem sikerült. Próbáld újra.","statusWhere":"{app} a beállításait az Unraid felületén tartja ezen az eszközön: {device}.","statusLoading":"Megkérdezzük, hogy van {app}.","statusQuiet":"Most semmi nem vár rád, és semmi sem fut.","statusNeedsYou":"Rád vár","statusWork":"Most fut","statusNext":"Következik","statusLog":"Legutóbbi bejegyzések","states":{"off":"Ki","ok":"Minden rendben","starting":"Indul","warning":"Érdemes ránézni","overdue":"Késésben","failed":"Sikertelen"},"levels":{"warn":"Figyelmeztetés","error":"Hiba"}},
    "id": {"fortress":"Benteng","showOthers":"Tampilkan aplikasi lain di benteng","about":"Di dalam benteng, aplikasi kamu saling melihat dan saling meringankan pekerjaan. Aplikasi bisa masuk dengan frasa.","othersApps":{"other":"{count} aplikasi lain"},"othersDevices":{"other":"{count} perangkat"},"othersLine":"{apps} di {devices}. Hanya berlaku untuk perangkat ini.","connected":"Terhubung","away":"Tidak dapat dijangkau","viaRelay":"Lewat relay","lastSeen":"Terakhir terlihat {time}","instanceCount":{"other":"{count} instance"},"forms":{"container":"Container","plugin":"Plugin Unraid","desktop":"Aplikasi desktop","android":"Aplikasi Android","extension":"Ekstensi browser"},"open":"Buka","remove":"Hapus","memberOn":"{app} di {device}","outHint":"Gabung ke benteng, lalu instance {app} lainnya akan muncul di sini, juga aplikasi lain kalau kamu mau.","removedHereTitle":"Dihapus dari benteng","removedHereHint":"Aplikasi lain di benteng menghapus {app}. Masukkan frasa yang berlaku sekarang untuk masuk lagi.","enterPhrase":"Masukkan frasa","joinTitle":"Gabung ke benteng","word":"Kata {n}","phraseWhere":"Setiap aplikasi di benteng menampilkan frasa di bagian Instance. Tempel ke kolom pertama, nanti kata-katanya tersebar ke kolom lainnya.","join":"Gabung","joinRefused":"Kata-kata ini tidak membuka benteng mana pun. Periksa lalu coba lagi.","joinedTitle":"{app} sekarang ada di benteng","cancel":"Batal","close":"Tutup","leaveTitle":"Keluar dari benteng?","leaveBody":"Aplikasi lain tidak akan melihat {app} lagi, dan semua izin dengan {app} berakhir.","leaveBack":"Kamu bisa kembali dengan frasa yang berlaku sekarang.","leaveConfirm":"Keluar","removeTitle":"Hapus {device}?","removeBody":"{device} langsung kehilangan aksesnya. Ini tidak bisa dibatalkan.","removeBack":"Siapa pun yang masih punya frasa bisa masuk lagi sampai kamu mengubahnya.","removedTitle":"Ubah frasa sekarang?","removedBody":"{device} sudah di luar. Selama frasanya tetap sama, jalan kembali masih terbuka.","removedNote":"Dihapus: {devices}. Ubah frasa untuk menutup jalan kembali.","phraseTitle":"Frasa","leaveRow":"Aplikasi ini dalam benteng","leaveAbout":"Mengeluarkan {app} dari benteng. Dengan frasa yang berlaku sekarang, aplikasi itu bisa kembali.","changedTitle":"Frasa baru","changeElsewhere":"Kamu mengubah frasa di {apps}.","changeNowhere":"{app} tidak menyimpan frasa. Ubah frasa di aplikasi benteng yang menampilkannya.","phraseMoved":"Bentengmu punya frasa baru yang tidak bisa diikuti {app} sendiri. Masukkan kata-kata baru dan ia tetap menjadi anggota yang sama.","failed":"Belum berhasil. Silakan coba lagi.","statusWhere":"{app} menyimpan pengaturannya di Unraid pada {device}.","statusLoading":"Menanyakan kabar {app}.","statusQuiet":"Saat ini tidak ada yang butuh kamu, dan tidak ada yang berjalan.","statusNeedsYou":"Butuh kamu","statusWork":"Sedang berjalan","statusNext":"Berikutnya","statusLog":"Entri terbaru","states":{"off":"Mati","ok":"Semua beres","starting":"Memulai","warning":"Perlu dicek","overdue":"Terlambat","failed":"Gagal"},"levels":{"warn":"Peringatan","error":"Kesalahan"}},
    "is": {"fortress":"Vígi","showOthers":"Sýna önnur forrit í víginu","about":"Í vígi sjá forritin þín hvert annað og hjálpast að við verkin. Forrit kemst inn með setningunni.","othersApps":{"one":"{count} forrit í viðbót","other":"{count} forrit í viðbót"},"othersDevices":{"one":"{count} tæki","other":"{count} tækjum"},"othersLine":"{apps} á {devices}. Gildir aðeins fyrir þetta tæki.","connected":"Tengt","away":"Ekki aðgengilegt","viaRelay":"Gegnum relay","lastSeen":"Síðast séð {time}","instanceCount":{"one":"{count} tilvik","other":"{count} tilvik"},"forms":{"container":"Gámur","plugin":"Unraid-viðbót","desktop":"Skjáborðsforrit","android":"Android-forrit","extension":"Vafraviðbót"},"open":"Opna","remove":"Fjarlægja","memberOn":"{app} á {device}","outHint":"Gakktu í vígi, þá birtast hér önnur tilvik forritsins {app} og hin forritin líka ef þú vilt.","removedHereTitle":"Fjarlægt úr víginu","removedHereHint":"Annað forrit í víginu fjarlægði {app}. Sláðu inn núgildandi setningu til að komast aftur inn.","enterPhrase":"Slá inn setningu","joinTitle":"Ganga í vígi","word":"Orð {n}","phraseWhere":"Öll forrit í víginu sýna setninguna undir Tilvik. Límdu hana í fyrsta reitinn og þá dreifist hún á hina.","join":"Ganga í","joinRefused":"Þessi orð opna ekkert vígi. Athugaðu þau og reyndu aftur.","joinedTitle":"{app} er nú í víginu","cancel":"Hætta við","close":"Loka","leaveTitle":"Yfirgefa vígið?","leaveBody":"Hin forritin sjá ekki lengur forritið {app}, og allar heimildir forritsins {app} falla niður.","leaveBack":"Þú kemst aftur inn með núgildandi setningu.","leaveConfirm":"Yfirgefa","removeTitle":"Fjarlægja tilvikið {device}?","removeBody":"{device} missir aðganginn strax. Það er ekki hægt að afturkalla þetta.","removeBack":"Allir sem enn hafa setninguna komast aftur inn þar til þú breytir henni.","removedTitle":"Breyta setningunni núna?","removedBody":"{device} er úti. Á meðan setningin helst sú sama er leiðin til baka samt enn opin.","removedNote":"Fjarlægt: {devices}. Breyttu setningunni til að loka leiðinni til baka.","phraseTitle":"Setning","leaveRow":"Þetta forrit í víginu","leaveAbout":"Tekur forritið {app} úr víginu. Með núgildandi setningu kemst það aftur inn.","changedTitle":"Ný setning","changeElsewhere":"Setningunni breytir þú í {apps}.","changeNowhere":"{app} geymir ekki setninguna. Breyttu henni í forriti í víginu sem sýnir hana.","phraseMoved":"Vígið þitt er með nýja setningu sem {app} gat ekki fylgt sjálft. Sláðu inn nýju orðin og það verður áfram sami meðlimur.","failed":"Það tókst ekki. Reyndu aftur.","statusWhere":"{app} geymir stillingarnar sínar í Unraid á tækinu {device}.","statusLoading":"Spyr {app} hvernig gengur.","statusQuiet":"Ekkert þarf á þér að halda núna og ekkert er í gangi.","statusNeedsYou":"Þarf á þér að halda","statusWork":"Í gangi núna","statusNext":"Næst á dagskrá","statusLog":"Nýjustu færslur","states":{"off":"Slökkt","ok":"Allt í lagi","starting":"Ræsist","warning":"Þarf að skoða","overdue":"Á eftir áætlun","failed":"Mistókst"},"levels":{"warn":"Viðvörun","error":"Villa"}},
    "it": {"fortress":"Fortezza","showOthers":"Mostra le altre app della fortezza","about":"In una fortezza le tue app si vedono a vicenda e si dividono il lavoro. Un'app entra con la frase.","othersApps":{"one":"{count} altra app","many":"{count} altre app","other":"{count} altre app"},"othersDevices":{"one":"{count} dispositivo","many":"{count} dispositivi","other":"{count} dispositivi"},"othersLine":"{apps} su {devices}. Vale solo per questo dispositivo.","connected":"Connesso","away":"Non raggiungibile","viaRelay":"Tramite relay","lastSeen":"Ultima attività {time}","instanceCount":{"one":"{count} istanza","many":"{count} istanze","other":"{count} istanze"},"forms":{"container":"Container","plugin":"Plugin Unraid","desktop":"App desktop","android":"App Android","extension":"Estensione del browser"},"open":"Apri","remove":"Rimuovi","memberOn":"{app} su {device}","outHint":"Entra in una fortezza e qui compariranno le tue altre istanze di {app}, e se vuoi anche le altre app.","removedHereTitle":"Rimosso dalla fortezza","removedHereHint":"Un'altra app della fortezza ha rimosso {app}. Inserisci la frase attuale per rientrare.","enterPhrase":"Inserisci la frase","joinTitle":"Entra in una fortezza","word":"Parola {n}","phraseWhere":"Ogni app della fortezza mostra la frase in Istanze. Incollala nel primo campo e si distribuisce sugli altri.","join":"Entra","joinRefused":"Queste parole non aprono nessuna fortezza. Controllale e riprova.","joinedTitle":"{app} ora è nella fortezza","cancel":"Annulla","close":"Chiudi","leaveTitle":"Lasciare la fortezza?","leaveBody":"Le altre app non vedranno più {app}, e tutti i permessi con {app} terminano.","leaveBack":"Puoi tornare con la frase attuale.","leaveConfirm":"Lascia","removeTitle":"Rimuovere {device}?","removeBody":"{device} perde subito l'accesso. Non si può annullare.","removeBack":"Chi ha ancora la frase può rientrare finché non la cambi.","removedTitle":"Cambiare la frase adesso?","removedBody":"{device} è fuori. Finché la frase resta la stessa, la via per rientrare è ancora aperta.","removedNote":"Rimossi: {devices}. Cambia la frase per chiudere la via del ritorno.","phraseTitle":"Frase","leaveRow":"Questa app nella fortezza","leaveAbout":"Porta {app} fuori dalla fortezza. Con la frase attuale può tornare.","changedTitle":"Nuova frase","changeElsewhere":"La frase la cambi in {apps}.","changeNowhere":"{app} non conserva la frase. Cambiala in un'app della fortezza che la mostra.","phraseMoved":"La tua fortezza ha una nuova frase che {app} non è riuscito a seguire da solo. Inserisci le nuove parole e resterà lo stesso membro.","failed":"Non ha funzionato. Riprova.","statusWhere":"{app} tiene le sue impostazioni in Unraid su {device}.","statusLoading":"Chiedo a {app} come sta.","statusQuiet":"Al momento niente ha bisogno di te e non c'è niente in esecuzione.","statusNeedsYou":"Ha bisogno di te","statusWork":"In corso","statusNext":"In arrivo","statusLog":"Ultime voci","states":{"off":"Spento","ok":"Tutto ok","starting":"In avvio","warning":"Da controllare","overdue":"In ritardo","failed":"Non riuscito"},"levels":{"warn":"Avviso","error":"Errore"}},
    "ja": {"fortress":"要塞","showOthers":"要塞のほかのアプリを表示","about":"要塞の中では、アプリどうしがお互いを認識して、作業を分担し合います。アプリはフレーズで参加できます。","othersApps":{"other":"ほかのアプリ{count}個"},"othersDevices":{"other":"{count}台のデバイス"},"othersLine":"{devices}上の{apps}。この設定はこのデバイスにだけ適用されます。","connected":"接続済み","away":"到達できません","viaRelay":"リレー経由","lastSeen":"最終確認 {time}","instanceCount":{"other":"{count}個のインスタンス"},"forms":{"container":"コンテナ","plugin":"Unraid プラグイン","desktop":"デスクトップアプリ","android":"Android アプリ","extension":"ブラウザ拡張機能"},"open":"開く","remove":"削除","memberOn":"{device}の{app}","outHint":"要塞に参加すると、{app}のほかのインスタンスがここに表示されます。必要ならほかのアプリも一緒に表示できます。","removedHereTitle":"要塞から削除されました","removedHereHint":"要塞の別のアプリが{app}を削除しました。現在のフレーズを入力すると戻れます。","enterPhrase":"フレーズを入力","joinTitle":"要塞に参加","word":"単語 {n}","phraseWhere":"フレーズは、要塞内のどのアプリでも「インスタンス」の下に表示されます。最初の欄に貼り付けると、残りの欄にも自動で入ります。","join":"参加","joinRefused":"この単語では要塞を開けません。確認して、もう一度お試しください。","joinedTitle":"{app}が要塞に参加しました","cancel":"キャンセル","close":"閉じる","leaveTitle":"要塞から退出しますか？","leaveBody":"ほかのアプリから{app}が見えなくなり、{app}に関わる許可もすべて終了します。","leaveBack":"現在のフレーズがあれば戻れます。","leaveConfirm":"退出","removeTitle":"{device}を削除しますか？","removeBody":"{device}はすぐにアクセスできなくなります。元に戻すことはできません。","removeBack":"フレーズを変更するまでは、フレーズを知っている人なら誰でも再び参加できます。","removedTitle":"今すぐフレーズを変更しますか？","removedBody":"{device}は外れました。ただしフレーズが同じままなら、戻る道はまだ開いています。","removedNote":"削除済み：{devices}。フレーズを変更すると、戻る道が閉じます。","phraseTitle":"フレーズ","leaveRow":"要塞内のこのアプリ","leaveAbout":"{app}を要塞から外します。現在のフレーズがあれば戻れます。","changedTitle":"新しいフレーズ","changeElsewhere":"フレーズは {apps} で変更できます。","changeNowhere":"{app}はフレーズを保持しません。フレーズを表示できる要塞内のアプリで変更してください。","phraseMoved":"要塞のフレーズが新しくなりましたが、{app}は自動で追従できませんでした。新しい単語を入力すると、同じメンバーのまま続けられます。","failed":"うまくいきませんでした。もう一度お試しください。","statusWhere":"{app}の設定は、{device}のUnraidにあります。","statusLoading":"{app}の状態を確認しています。","statusQuiet":"今は対応が必要なものも、実行中のものもありません。","statusNeedsYou":"対応が必要","statusWork":"実行中","statusNext":"次の予定","statusLog":"最新のログ","states":{"off":"オフ","ok":"問題なし","starting":"起動中","warning":"確認が必要","overdue":"予定超過","failed":"失敗"},"levels":{"warn":"警告","error":"エラー"}},
    "ko": {"fortress":"요새","showOthers":"요새의 다른 앱 보기","about":"요새 안에서는 앱끼리 서로를 보고 일을 나눠 맡아요. 앱은 문구로 들어올 수 있어요.","othersApps":{"other":"다른 앱 {count}개"},"othersDevices":{"other":"기기 {count}대"},"othersLine":"{devices}에 있는 {apps}. 이 기기에만 적용돼요.","connected":"연결됨","away":"연결할 수 없음","viaRelay":"릴레이 경유","lastSeen":"마지막 확인 {time}","instanceCount":{"other":"인스턴스 {count}개"},"forms":{"container":"컨테이너","plugin":"Unraid 플러그인","desktop":"데스크톱 앱","android":"Android 앱","extension":"브라우저 확장 프로그램"},"open":"열기","remove":"제거","memberOn":"{device}의 {app}","outHint":"요새에 참여하면 {app}의 다른 인스턴스가 여기에 나타나요. 원하면 다른 앱도 함께 볼 수 있어요.","removedHereTitle":"요새에서 제거됨","removedHereHint":"요새의 다른 앱이 {app}을(를) 제거했어요. 현재 문구를 입력하면 다시 들어올 수 있어요.","enterPhrase":"문구 입력","joinTitle":"요새 참여하기","word":"단어 {n}","phraseWhere":"문구는 요새에 있는 모든 앱의 인스턴스 화면에 있어요. 첫 번째 칸에 붙여 넣으면 나머지 칸에도 나눠서 들어가요.","join":"참여","joinRefused":"이 단어로는 요새를 열 수 없어요. 확인하고 다시 시도해 주세요.","joinedTitle":"이제 {app}도 요새에 있어요","cancel":"취소","close":"닫기","leaveTitle":"요새에서 나갈까요?","leaveBody":"다른 앱의 목록에서 {app} 항목이 사라지고, {app}에 대한 권한도 모두 끝나요.","leaveBack":"현재 문구가 있으면 다시 돌아올 수 있어요.","leaveConfirm":"나가기","removeTitle":"{device} 기기를 제거할까요?","removeBody":"{device} 기기는 곧바로 접근할 수 없게 돼요. 되돌릴 수 없어요.","removeBack":"문구를 바꾸기 전까지는 문구를 아는 사람이면 누구나 다시 들어올 수 있어요.","removedTitle":"지금 문구를 바꿀까요?","removedBody":"{device} 기기는 이제 빠졌어요. 하지만 문구가 그대로인 동안에는 다시 돌아올 길이 열려 있어요.","removedNote":"제거됨: {devices}. 문구를 바꾸면 돌아올 길이 닫혀요.","phraseTitle":"문구","leaveRow":"요새 안의 이 앱","leaveAbout":"{app} 앱을 요새에서 빼요. 현재 문구가 있으면 다시 돌아올 수 있어요.","changedTitle":"새 문구","changeElsewhere":"문구는 {apps}에서 바꿔요.","changeNowhere":"{app}은(는) 문구를 보관하지 않아요. 문구를 보여 주는 요새의 앱에서 바꿔 주세요.","phraseMoved":"요새의 문구가 새로 바뀌었는데 {app}이(가) 스스로 따라가지 못했어요. 새 단어를 입력하면 같은 구성원으로 계속돼요.","failed":"잘 안 됐어요. 다시 시도해 주세요.","statusWhere":"{app} 설정은 {device}의 Unraid에 있어요.","statusLoading":"{app} 상태를 확인하고 있어요.","statusQuiet":"지금은 확인할 일도, 실행 중인 작업도 없어요.","statusNeedsYou":"확인 필요","statusWork":"실행 중","statusNext":"다음 예정","statusLog":"최근 기록","states":{"off":"꺼짐","ok":"모두 정상","starting":"시작 중","warning":"확인 필요","overdue":"기한 지남","failed":"실패"},"levels":{"warn":"경고","error":"오류"}},
    "lt": {"fortress":"Tvirtovė","showOthers":"Rodyti kitas tvirtovės programėles","about":"Tvirtovėje jūsų programėlės mato viena kitą ir pasidalija darbą. Programėlė prisijungia su fraze.","othersApps":{"one":"dar {count} programėlė","few":"dar {count} programėlės","many":"dar {count} programėlės","other":"dar {count} programėlių"},"othersDevices":{"one":"{count} įrenginyje","few":"{count} įrenginiuose","many":"{count} įrenginio","other":"{count} įrenginių"},"othersLine":"{apps} {devices}. Taikoma tik šiam įrenginiui.","connected":"Prisijungta","away":"Nepasiekiama","viaRelay":"Per relay","lastSeen":"Paskutinį kartą matytas {time}","instanceCount":{"one":"{count} egzempliorius","few":"{count} egzemplioriai","many":"{count} egzemplioriaus","other":"{count} egzempliorių"},"forms":{"container":"Konteineris","plugin":"Unraid papildinys","desktop":"Darbalaukio programėlė","android":"Android programėlė","extension":"Naršyklės plėtinys"},"open":"Atidaryti","remove":"Pašalinti","memberOn":"{app} įrenginyje {device}","outHint":"Prisijunkite prie tvirtovės ir čia matysite kitus programėlės {app} egzempliorius, o jei norite, ir kitas programėles.","removedHereTitle":"Pašalinta iš tvirtovės","removedHereHint":"Kita tvirtovės programa pašalino {app}. Įveskite dabartinę frazę ir grįšite.","enterPhrase":"Įvesti frazę","joinTitle":"Prisijungti prie tvirtovės","word":"Žodis {n}","phraseWhere":"Kiekviena tvirtovės programėlė rodo frazę skiltyje Egzemplioriai. Įklijuokite ją į pirmą lauką, ir ji pasiskirstys po kitus.","join":"Prisijungti","joinRefused":"Šie žodžiai neatveria jokios tvirtovės. Patikrinkite juos ir bandykite dar kartą.","joinedTitle":"Programėlė {app} dabar tvirtovėje","cancel":"Atšaukti","close":"Uždaryti","leaveTitle":"Palikti tvirtovę?","leaveBody":"Kitos programėlės nebematys programėlės {app}, o visi programėlės {app} leidimai baigsis.","leaveBack":"Su dabartine fraze galite grįžti.","leaveConfirm":"Palikti","removeTitle":"Pašalinti egzempliorių {device}?","removeBody":"{device} iš karto netenka prieigos. To atšaukti negalima.","removeBack":"Kas dar turi frazę, gali grįžti, kol jos nepakeisite.","removedTitle":"Pakeisti frazę dabar?","removedBody":"{device} jau išorėje. Kol frazė nepasikeičia, kelias atgal vis dar atviras.","removedNote":"Pašalinta: {devices}. Pakeiskite frazę, kad kelias atgal užsivertų.","phraseTitle":"Frazė","leaveRow":"Ši programėlė tvirtovėje","leaveAbout":"Išima programėlę {app} iš tvirtovės. Su dabartine fraze ji gali grįžti.","changedTitle":"Nauja frazė","changeElsewhere":"Frazę keičiate programoje {apps}.","changeNowhere":"{app} frazės nesaugo. Pakeiskite ją tvirtovės programoje, kuri ją rodo.","phraseMoved":"Jūsų tvirtovė turi naują frazę, kurios {app} negalėjo perimti pats. Įveskite naujus žodžius ir narys liks tas pats.","failed":"Nepavyko. Bandykite dar kartą.","statusWhere":"{app} nustatymai saugomi Unraid įrenginyje {device}.","statusLoading":"Tikriname {app} būseną.","statusQuiet":"Šiuo metu niekas nelaukia jūsų dėmesio ir niekas nevykdoma.","statusNeedsYou":"Laukia jūsų","statusWork":"Vyksta dabar","statusNext":"Netrukus","statusLog":"Naujausi įrašai","states":{"off":"Išjungta","ok":"Viskas gerai","starting":"Paleidžiama","warning":"Verta pažiūrėti","overdue":"Vėluoja","failed":"Nepavyko"},"levels":{"warn":"Įspėjimas","error":"Klaida"}},
    "lv": {"fortress":"Cietoksnis","showOthers":"Rādīt citas cietokšņa lietotnes","about":"Cietoksnī tavas lietotnes redz cita citu un palīdz cita citai darbā. Lietotni var pievienot ar frāzi.","othersApps":{"zero":"vēl {count} lietotnes","one":"vēl {count} lietotne","other":"vēl {count} lietotnes"},"othersDevices":{"zero":"{count} ierīcēs","one":"{count} ierīcē","other":"{count} ierīcēs"},"othersLine":"{apps} {devices}. Attiecas tikai uz šo ierīci.","connected":"Savienots","away":"Nav sasniedzams","viaRelay":"Caur relay","lastSeen":"Pēdējoreiz redzēts {time}","instanceCount":{"zero":"{count} instances","one":"{count} instance","other":"{count} instances"},"forms":{"container":"Konteiners","plugin":"Unraid spraudnis","desktop":"Darbvirsmas lietotne","android":"Android lietotne","extension":"Pārlūka paplašinājums"},"open":"Atvērt","remove":"Noņemt","memberOn":"{app} ierīcē {device}","outHint":"Pievienojies cietoksnim, un šeit parādīsies lietotnes {app} pārējās instances un, ja vēlies, arī citas lietotnes.","removedHereTitle":"Noņemts no cietokšņa","removedHereHint":"Cita cietokšņa lietotne noņēma {app}. Ievadi pašreizējo frāzi, lai atgrieztos.","enterPhrase":"Ievadīt frāzi","joinTitle":"Pievienoties cietoksnim","word":"Vārds {n}","phraseWhere":"Katra cietokšņa lietotne rāda frāzi sadaļā Instances. Ielīmē to pirmajā laukā, un tā sadalīsies pa pārējiem.","join":"Pievienoties","joinRefused":"Šie vārdi neatver nevienu cietoksni. Pārbaudi tos un mēģini vēlreiz.","joinedTitle":"Lietotne {app} tagad ir cietoksnī","cancel":"Atcelt","close":"Aizvērt","leaveTitle":"Pamest cietoksni?","leaveBody":"Citas lietotnes vairs neredzēs lietotni {app}, un visas lietotnes {app} atļaujas beigsies.","leaveBack":"Ar pašreizējo frāzi vari atgriezties.","leaveConfirm":"Pamest","removeTitle":"Noņemt instanci {device}?","removeBody":"{device} uzreiz zaudē piekļuvi. To nevar atsaukt.","removeBack":"Ikviens, kam vēl ir frāze, var atgriezties, līdz tu to nomaini.","removedTitle":"Nomainīt frāzi tagad?","removedBody":"{device} ir ārā. Kamēr frāze paliek tā pati, ceļš atpakaļ vēl ir vaļā.","removedNote":"Noņemti: {devices}. Nomaini frāzi, lai aizvērtu ceļu atpakaļ.","phraseTitle":"Frāze","leaveRow":"Šī lietotne cietoksnī","leaveAbout":"Izņem lietotni {app} no cietokšņa. Ar pašreizējo frāzi tā var atgriezties.","changedTitle":"Jaunā frāze","changeElsewhere":"Frāzi tu maini lietotnē {apps}.","changeNowhere":"{app} frāzi neglabā. Maini to cietokšņa lietotnē, kas to rāda.","phraseMoved":"Tavam cietoksnim ir jauna frāze, kurai {app} pats nespēja sekot. Ievadi jaunos vārdus, un tas paliks tas pats dalībnieks.","failed":"Neizdevās. Mēģini vēlreiz.","statusWhere":"Lietotnes {app} iestatījumi ir Unraid ierīcē {device}.","statusLoading":"Jautājam lietotnei {app}, kā tai klājas.","statusQuiet":"Pašlaik nekas tevi negaida un nekas nedarbojas.","statusNeedsYou":"Gaida tevi","statusWork":"Notiek tagad","statusNext":"Drīzumā","statusLog":"Jaunākie ieraksti","states":{"off":"Izslēgts","ok":"Viss kārtībā","starting":"Startē","warning":"Vajag ieskatīties","overdue":"Nokavēts","failed":"Neizdevās"},"levels":{"warn":"Brīdinājums","error":"Kļūda"}},
    "ms": {"fortress":"Kubu","showOthers":"Tunjukkan apl lain dalam kubu","about":"Dalam kubu, apl anda saling melihat dan saling meringankan kerja. Apl boleh masuk dengan frasa.","othersApps":{"other":"{count} apl lagi"},"othersDevices":{"other":"{count} peranti"},"othersLine":"{apps} pada {devices}. Hanya untuk peranti ini.","connected":"Disambungkan","away":"Tidak dapat dicapai","viaRelay":"Melalui relay","lastSeen":"Kali terakhir dilihat {time}","instanceCount":{"other":"{count} instans"},"forms":{"container":"Container","plugin":"Plugin Unraid","desktop":"Apl desktop","android":"Apl Android","extension":"Sambungan penyemak imbas"},"open":"Buka","remove":"Alih keluar","memberOn":"{app} pada {device}","outHint":"Sertai kubu dan instans {app} anda yang lain akan dipaparkan di sini, bersama apl lain jika anda mahu.","removedHereTitle":"Dialih keluar daripada kubu","removedHereHint":"Apl lain dalam kubu telah mengalih keluar {app}. Masukkan frasa semasa untuk masuk semula.","enterPhrase":"Masukkan frasa","joinTitle":"Sertai kubu","word":"Perkataan {n}","phraseWhere":"Setiap apl dalam kubu memaparkan frasa di bawah Instans. Tampal ke dalam medan pertama dan perkataan selebihnya akan diisi ke medan lain.","join":"Sertai","joinRefused":"Perkataan ini tidak membuka mana-mana kubu. Semak dan cuba lagi.","joinedTitle":"{app} kini dalam kubu","cancel":"Batal","close":"Tutup","leaveTitle":"Keluar dari kubu?","leaveBody":"Apl lain tidak akan melihat {app} lagi, dan semua kebenaran dengan {app} tamat.","leaveBack":"Anda boleh kembali dengan frasa semasa.","leaveConfirm":"Keluar","removeTitle":"Alih keluar {device}?","removeBody":"{device} terus hilang aksesnya. Tindakan ini tidak boleh dibuat asal.","removeBack":"Sesiapa yang masih ada frasa boleh masuk semula sehingga anda menukarnya.","removedTitle":"Tukar frasa sekarang?","removedBody":"{device} sudah di luar. Selagi frasa kekal sama, jalan masuk semula masih terbuka.","removedNote":"Dialih keluar: {devices}. Tukar frasa untuk menutup jalan masuk semula.","phraseTitle":"Frasa","leaveRow":"Apl ini dalam kubu","leaveAbout":"Mengeluarkan {app} dari kubu. Dengan frasa semasa, apl itu boleh kembali.","changedTitle":"Frasa baharu","changeElsewhere":"Anda menukar frasa dalam {apps}.","changeNowhere":"{app} tidak menyimpan frasa. Tukar frasa dalam apl kubu yang memaparkannya.","phraseMoved":"Kubu anda mempunyai frasa baharu yang tidak dapat diikuti oleh {app} sendiri. Masukkan perkataan baharu dan ia kekal sebagai ahli yang sama.","failed":"Tidak berjaya. Sila cuba lagi.","statusWhere":"{app} menyimpan tetapannya dalam Unraid pada {device}.","statusLoading":"Bertanya khabar kepada {app}.","statusQuiet":"Buat masa ini tiada apa-apa yang memerlukan anda, dan tiada apa-apa yang berjalan.","statusNeedsYou":"Memerlukan anda","statusWork":"Sedang berjalan","statusNext":"Akan datang","statusLog":"Entri terkini","states":{"off":"Mati","ok":"Semuanya baik","starting":"Sedang bermula","warning":"Perlu dilihat","overdue":"Lewat","failed":"Gagal"},"levels":{"warn":"Amaran","error":"Ralat"}},
    "nl": {"fortress":"Vesting","showOthers":"Toon de andere apps van de vesting","about":"In een vesting zien je apps elkaar en nemen ze elkaar werk uit handen. Een app komt binnen met de frase.","othersApps":{"one":"nog {count} app","other":"nog {count} apps"},"othersDevices":{"one":"{count} apparaat","other":"{count} apparaten"},"othersLine":"{apps} op {devices}. Geldt alleen voor dit apparaat.","connected":"Verbonden","away":"Niet bereikbaar","viaRelay":"Via relay","lastSeen":"Laatst gezien {time}","instanceCount":{"one":"{count} instantie","other":"{count} instanties"},"forms":{"container":"Container","plugin":"Unraid-plug-in","desktop":"Desktop-app","android":"Android-app","extension":"Browserextensie"},"open":"Openen","remove":"Verwijderen","memberOn":"{app} op {device}","outHint":"Sluit je aan bij een vesting, dan zie je hier je andere instanties van {app} en, als je wilt, ook de andere apps.","removedHereTitle":"Uit de vesting verwijderd","removedHereHint":"Een andere app in de vesting heeft {app} verwijderd. Voer de huidige frase in om weer binnen te komen.","enterPhrase":"Frase invoeren","joinTitle":"Aansluiten bij een vesting","word":"Woord {n}","phraseWhere":"Elke app in de vesting toont de frase onder Instanties. Plak de frase in het eerste veld, dan verdelen de woorden zich over de rest.","join":"Aansluiten","joinRefused":"Deze woorden openen geen vesting. Controleer ze en probeer het opnieuw.","joinedTitle":"{app} zit nu in de vesting","cancel":"Annuleren","close":"Sluiten","leaveTitle":"Vesting verlaten?","leaveBody":"De andere apps zien {app} dan niet meer, en alle toestemmingen met {app} vervallen.","leaveBack":"Met de huidige frase kom je terug.","leaveConfirm":"Verlaten","removeTitle":"{device} verwijderen?","removeBody":"{device} verliest meteen zijn toegang. Dit kun je niet ongedaan maken.","removeBack":"Wie de frase nog heeft, kan weer binnenkomen tot je de frase wijzigt.","removedTitle":"Frase nu wijzigen?","removedBody":"{device} is eruit. Zolang de frase hetzelfde blijft, staat de weg terug nog open.","removedNote":"Verwijderd: {devices}. Wijzig de frase om de weg terug te sluiten.","phraseTitle":"Frase","leaveRow":"Deze app in de vesting","leaveAbout":"Haalt {app} uit de vesting. Met de huidige frase komt die terug.","changedTitle":"Nieuwe frase","changeElsewhere":"De frase wijzig je in {apps}.","changeNowhere":"{app} bewaart de frase niet. Wijzig hem in een app van de vesting die hem toont.","phraseMoved":"Je vesting heeft een nieuwe frase die {app} niet zelf kon volgen. Voer de nieuwe woorden in, dan blijft het hetzelfde lid.","failed":"Dat is niet gelukt. Probeer het opnieuw.","statusWhere":"{app} bewaart zijn instellingen in Unraid op {device}.","statusLoading":"Even vragen hoe het met {app} gaat.","statusQuiet":"Er wacht nu niets op je en er loopt niets.","statusNeedsYou":"Wacht op je","statusWork":"Loopt nu","statusNext":"Hierna","statusLog":"Laatste meldingen","states":{"off":"Uit","ok":"Alles goed","starting":"Start op","warning":"Even kijken","overdue":"Te laat","failed":"Mislukt"},"levels":{"warn":"Waarschuwing","error":"Fout"}},
    "no": {"fortress":"Festning","showOthers":"Vis de andre appene i festningen","about":"I en festning ser appene dine hverandre og deler på arbeidet. En app kommer inn med frasen.","othersApps":{"one":"{count} annen app","other":"{count} andre apper"},"othersDevices":{"one":"{count} enhet","other":"{count} enheter"},"othersLine":"{apps} på {devices}. Gjelder bare denne enheten.","connected":"Tilkoblet","away":"Ikke tilgjengelig","viaRelay":"Via relay","lastSeen":"Sist sett {time}","instanceCount":{"one":"{count} instans","other":"{count} instanser"},"forms":{"container":"Container","plugin":"Unraid-plugin","desktop":"Skrivebordsapp","android":"Android-app","extension":"Nettleserutvidelse"},"open":"Åpne","remove":"Fjern","memberOn":"{app} på {device}","outHint":"Bli med i en festning, så vises de andre instansene dine av {app} her, og de andre appene også hvis du vil.","removedHereTitle":"Fjernet fra festningen","removedHereHint":"En annen app i festningen har fjernet {app}. Skriv inn den gjeldende frasen for å komme inn igjen.","enterPhrase":"Skriv inn frase","joinTitle":"Bli med i en festning","word":"Ord {n}","phraseWhere":"Alle appene i festningen viser frasen under Instanser. Lim den inn i det første feltet, så fordeler den seg på resten.","join":"Bli med","joinRefused":"Disse ordene åpner ingen festning. Sjekk dem og prøv igjen.","joinedTitle":"{app} er nå i festningen","cancel":"Avbryt","close":"Lukk","leaveTitle":"Forlate festningen?","leaveBody":"De andre appene ser ikke {app} lenger, og alle tillatelser med {app} opphører.","leaveBack":"Du kommer tilbake med den gjeldende frasen.","leaveConfirm":"Forlat","removeTitle":"Fjerne {device}?","removeBody":"{device} mister tilgangen med en gang. Dette kan ikke angres.","removeBack":"Alle som fortsatt har frasen, kan komme inn igjen til du bytter den.","removedTitle":"Bytte frasen nå?","removedBody":"{device} er ute. Så lenge frasen er den samme, står veien tilbake fortsatt åpen.","removedNote":"Fjernet: {devices}. Bytt frasen for å stenge veien tilbake.","phraseTitle":"Frase","leaveRow":"Denne appen i festningen","leaveAbout":"Tar {app} ut av festningen. Med den gjeldende frasen kommer den tilbake.","changedTitle":"Ny frase","changeElsewhere":"Frasen bytter du i {apps}.","changeNowhere":"{app} tar ikke vare på frasen. Bytt den i en app i festningen som viser den.","phraseMoved":"Festningen din har en ny frase som {app} ikke klarte å følge på egen hånd. Skriv inn de nye ordene, så fortsetter den som det samme medlemmet.","failed":"Det gikk ikke. Prøv igjen.","statusWhere":"{app} har innstillingene sine i Unraid på {device}.","statusLoading":"Spør {app} hvordan det går.","statusQuiet":"Ingenting venter på deg akkurat nå, og ingenting kjører.","statusNeedsYou":"Venter på deg","statusWork":"Kjører nå","statusNext":"Neste","statusLog":"Siste oppføringer","states":{"off":"Av","ok":"Alt i orden","starting":"Starter","warning":"Bør sjekkes","overdue":"Forsinket","failed":"Mislyktes"},"levels":{"warn":"Advarsel","error":"Feil"}},
    "pl": {"fortress":"Twierdza","showOthers":"Pokaż inne aplikacje twierdzy","about":"W twierdzy twoje aplikacje widzą się nawzajem i przejmują od siebie część pracy. Aplikacja dostaje się do niej dzięki frazie.","othersApps":{"one":"{count} inna aplikacja","few":"{count} inne aplikacje","many":"{count} innych aplikacji","other":"{count} innej aplikacji"},"othersDevices":{"one":"{count} urządzeniu","few":"{count} urządzeniach","many":"{count} urządzeniach","other":"{count} urządzenia"},"othersLine":"{apps} na {devices}. Dotyczy tylko tego urządzenia.","connected":"Połączona","away":"Nieosiągalna","viaRelay":"Przez relay","lastSeen":"Ostatnio widziana {time}","instanceCount":{"one":"{count} instancja","few":"{count} instancje","many":"{count} instancji","other":"{count} instancji"},"forms":{"container":"Kontener","plugin":"Wtyczka Unraid","desktop":"Aplikacja na komputer","android":"Aplikacja na Androida","extension":"Rozszerzenie przeglądarki"},"open":"Otwórz","remove":"Usuń","memberOn":"{app} na {device}","outHint":"Dołącz do twierdzy, a zobaczysz tu swoje pozostałe instancje aplikacji {app}, a jeśli chcesz, także inne aplikacje.","removedHereTitle":"Usunięto z twierdzy","removedHereHint":"Inna aplikacja w twierdzy usunęła {app}. Wpisz aktualną frazę, aby wrócić.","enterPhrase":"Wpisz frazę","joinTitle":"Dołącz do twierdzy","word":"Słowo {n}","phraseWhere":"Każda aplikacja w twierdzy pokazuje frazę w sekcji Instancje. Wklej ją w pierwsze pole, a sama rozłoży się na pozostałe.","join":"Dołącz","joinRefused":"Te słowa nie otwierają żadnej twierdzy. Sprawdź je i spróbuj ponownie.","joinedTitle":"Aplikacja {app} jest teraz w twierdzy","cancel":"Anuluj","close":"Zamknij","leaveTitle":"Opuścić twierdzę?","leaveBody":"Pozostałe aplikacje przestaną widzieć aplikację {app}, a wszystkie uprawnienia aplikacji {app} wygasną.","leaveBack":"Wrócić możesz dzięki aktualnej frazie.","leaveConfirm":"Opuść","removeTitle":"Usunąć instancję {device}?","removeBody":"{device} od razu traci dostęp. Tego nie da się cofnąć.","removeBack":"Każdy, kto nadal ma frazę, może wrócić, dopóki jej nie zmienisz.","removedTitle":"Zmienić frazę teraz?","removedBody":"{device} jest już poza twierdzą. Dopóki fraza się nie zmieni, droga powrotna wciąż stoi otworem.","removedNote":"Usunięto: {devices}. Zmień frazę, aby zamknąć drogę powrotną.","phraseTitle":"Fraza","leaveRow":"Ta aplikacja w twierdzy","leaveAbout":"Wyprowadza aplikację {app} z twierdzy. Dzięki aktualnej frazie może wrócić.","changedTitle":"Nowa fraza","changeElsewhere":"Frazę zmienisz w {apps}.","changeNowhere":"{app} nie przechowuje frazy. Zmień ją w aplikacji twierdzy, która ją pokazuje.","phraseMoved":"Twoja twierdza ma nową frazę, za którą {app} nie zdołał podążyć sam. Wpisz nowe słowa, a zostanie tym samym członkiem.","failed":"Nie udało się. Spróbuj ponownie.","statusWhere":"Ustawienia aplikacji {app} są w Unraid na urządzeniu {device}.","statusLoading":"Sprawdzamy, jak się ma {app}.","statusQuiet":"Teraz nic na ciebie nie czeka i nic nie jest uruchomione.","statusNeedsYou":"Czeka na ciebie","statusWork":"Trwa teraz","statusNext":"Wkrótce","statusLog":"Ostatnie wpisy","states":{"off":"Wyłączone","ok":"Wszystko gra","starting":"Uruchamia się","warning":"Warto zerknąć","overdue":"Zaległe","failed":"Niepowodzenie"},"levels":{"warn":"Ostrzeżenie","error":"Błąd"}},
    "pt": {"fortress":"Fortaleza","showOthers":"Mostrar as outras apps da fortaleza","about":"Numa fortaleza, as suas apps veem-se umas às outras e dividem o trabalho entre si. Uma app entra com a frase.","othersApps":{"one":"mais {count} app","many":"mais {count} apps","other":"mais {count} apps"},"othersDevices":{"one":"{count} dispositivo","many":"{count} dispositivos","other":"{count} dispositivos"},"othersLine":"{apps} em {devices}. Aplica-se só a este dispositivo.","connected":"Ligado","away":"Inacessível","viaRelay":"Via relay","lastSeen":"Visto pela última vez {time}","instanceCount":{"one":"{count} instância","many":"{count} instâncias","other":"{count} instâncias"},"forms":{"container":"Contentor","plugin":"Plugin do Unraid","desktop":"App de computador","android":"App Android","extension":"Extensão do navegador"},"open":"Abrir","remove":"Remover","memberOn":"{app} em {device}","outHint":"Junte-se a uma fortaleza e as suas outras instâncias de {app} aparecem aqui, e também as outras apps, se quiser.","removedHereTitle":"Removido da fortaleza","removedHereHint":"Outra app da fortaleza removeu {app}. Introduza a frase atual para voltar a entrar.","enterPhrase":"Introduzir frase","joinTitle":"Entrar numa fortaleza","word":"Palavra {n}","phraseWhere":"Todas as apps da fortaleza mostram a frase em Instâncias. Cole-a no primeiro campo e ela distribui-se pelos restantes.","join":"Entrar","joinRefused":"Estas palavras não abrem nenhuma fortaleza. Verifique-as e tente novamente.","joinedTitle":"{app} já está na fortaleza","cancel":"Cancelar","close":"Fechar","leaveTitle":"Sair da fortaleza?","leaveBody":"As outras apps deixam de ver {app}, e todas as permissões com {app} terminam.","leaveBack":"Pode voltar com a frase atual.","leaveConfirm":"Sair","removeTitle":"Remover {device}?","removeBody":"{device} perde o acesso de imediato. Isto não pode ser anulado.","removeBack":"Quem ainda tiver a frase pode voltar a entrar até a alterar.","removedTitle":"Alterar a frase agora?","removedBody":"{device} já está fora. Enquanto a frase se mantiver, o caminho de volta continua aberto.","removedNote":"Removidos: {devices}. Altere a frase para fechar o caminho de volta.","phraseTitle":"Frase","leaveRow":"Esta app na fortaleza","leaveAbout":"Tira {app} da fortaleza. Com a frase atual pode voltar.","changedTitle":"Frase nova","changeElsewhere":"A frase altera-se em {apps}.","changeNowhere":"O {app} não guarda a frase. Altere-a numa app da fortaleza que a mostre.","phraseMoved":"A sua fortaleza tem uma frase nova que o {app} não conseguiu acompanhar sozinho. Introduza as palavras novas e continua a ser o mesmo membro.","failed":"Não funcionou. Tente novamente.","statusWhere":"{app} guarda as definições no Unraid em {device}.","statusLoading":"A perguntar a {app} como está.","statusQuiet":"De momento nada precisa de si e nada está a correr.","statusNeedsYou":"Precisa de si","statusWork":"A correr agora","statusNext":"A seguir","statusLog":"Últimas entradas","states":{"off":"Desligado","ok":"Tudo bem","starting":"A iniciar","warning":"Precisa de atenção","overdue":"Em atraso","failed":"Falhou"},"levels":{"warn":"Aviso","error":"Erro"}},
    "ro": {"fortress":"Fortăreață","showOthers":"Arată celelalte aplicații din fortăreață","about":"Într-o fortăreață, aplicațiile tale se văd între ele și își împart munca. O aplicație intră cu fraza.","othersApps":{"one":"încă {count} aplicație","few":"încă {count} aplicații","other":"încă {count} de aplicații"},"othersDevices":{"one":"{count} dispozitiv","few":"{count} dispozitive","other":"{count} de dispozitive"},"othersLine":"{apps} pe {devices}. Se aplică doar pe acest dispozitiv.","connected":"Conectat","away":"Inaccesibil","viaRelay":"Prin relay","lastSeen":"Văzut ultima dată {time}","instanceCount":{"one":"{count} instanță","few":"{count} instanțe","other":"{count} de instanțe"},"forms":{"container":"Container","plugin":"Plugin Unraid","desktop":"Aplicație desktop","android":"Aplicație Android","extension":"Extensie de browser"},"open":"Deschide","remove":"Elimină","memberOn":"{app} pe {device}","outHint":"Intră într-o fortăreață și aici apar celelalte instanțe ale tale de {app} și, dacă vrei, celelalte aplicații.","removedHereTitle":"Eliminat din fortăreață","removedHereHint":"O altă aplicație din fortăreață a eliminat {app}. Introdu fraza actuală ca să intri din nou.","enterPhrase":"Introdu fraza","joinTitle":"Intră într-o fortăreață","word":"Cuvântul {n}","phraseWhere":"Fiecare aplicație din fortăreață arată fraza la Instanțe. Lipește-o în primul câmp și se împarte singură în celelalte.","join":"Intră","joinRefused":"Aceste cuvinte nu deschid nicio fortăreață. Verifică-le și încearcă din nou.","joinedTitle":"{app} este acum în fortăreață","cancel":"Anulează","close":"Închide","leaveTitle":"Părăsești fortăreața?","leaveBody":"Celelalte aplicații nu vor mai vedea {app}, iar toate permisiunile cu {app} se încheie.","leaveBack":"Poți reveni cu fraza actuală.","leaveConfirm":"Părăsește","removeTitle":"Elimini {device}?","removeBody":"{device} își pierde accesul imediat. Acțiunea nu poate fi anulată.","removeBack":"Oricine mai are fraza poate intra din nou până când o schimbi.","removedTitle":"Schimbi fraza acum?","removedBody":"{device} e afară. Cât timp fraza rămâne aceeași, drumul înapoi e încă deschis.","removedNote":"Eliminate: {devices}. Schimbă fraza ca să închizi drumul înapoi.","phraseTitle":"Frază","leaveRow":"Această aplicație din fortăreață","leaveAbout":"Scoate {app} din fortăreață. Cu fraza actuală poate reveni.","changedTitle":"Frază nouă","changeElsewhere":"Fraza o schimbi în {apps}.","changeNowhere":"{app} nu păstrează fraza. Schimb-o într-o aplicație din fortăreață care o arată.","phraseMoved":"Fortăreața ta are o frază nouă pe care {app} n-a putut s-o urmeze singur. Introdu cuvintele noi și rămâne același membru.","failed":"Nu a mers. Încearcă din nou.","statusWhere":"{app} își ține setările în Unraid pe {device}.","statusLoading":"Verificăm cum merge {app}.","statusQuiet":"Acum nimic nu are nevoie de tine și nu rulează nimic.","statusNeedsYou":"Are nevoie de tine","statusWork":"Rulează acum","statusNext":"Urmează","statusLog":"Ultimele intrări","states":{"off":"Oprit","ok":"Totul e bine","starting":"Pornește","warning":"Merită o privire","overdue":"Întârziat","failed":"Eșuat"},"levels":{"warn":"Avertisment","error":"Eroare"}},
    "ru": {"fortress":"Крепость","showOthers":"Показать другие приложения крепости","about":"В крепости ваши приложения видят друг друга и помогают друг другу с работой. Приложение попадает в неё с помощью фразы.","othersApps":{"one":"{count} другое приложение","few":"{count} других приложения","many":"{count} других приложений","other":"{count} другого приложения"},"othersDevices":{"one":"{count} устройстве","few":"{count} устройствах","many":"{count} устройствах","other":"{count} устройствах"},"othersLine":"{apps} на {devices}. Действует только на этом устройстве.","connected":"Подключено","away":"Недоступно","viaRelay":"Через relay","lastSeen":"Был в сети {time}","instanceCount":{"one":"{count} экземпляр","few":"{count} экземпляра","many":"{count} экземпляров","other":"{count} экземпляра"},"forms":{"container":"Контейнер","plugin":"Плагин Unraid","desktop":"Приложение для компьютера","android":"Приложение для Android","extension":"Расширение браузера"},"open":"Открыть","remove":"Удалить","memberOn":"{app} на {device}","outHint":"Вступите в крепость, и здесь появятся другие ваши экземпляры приложения {app}, а при желании и остальные приложения.","removedHereTitle":"Удалено из крепости","removedHereHint":"Другое приложение крепости удалило {app}. Введите текущую фразу, чтобы вернуться.","enterPhrase":"Ввести фразу","joinTitle":"Вступить в крепость","word":"Слово {n}","phraseWhere":"Фраза есть в каждом приложении крепости в разделе Экземпляры. Вставьте её в первое поле, и она сама распределится по остальным.","join":"Вступить","joinRefused":"Эти слова не открывают ни одну крепость. Проверьте их и попробуйте ещё раз.","joinedTitle":"Приложение {app} теперь в крепости","cancel":"Отмена","close":"Закрыть","leaveTitle":"Покинуть крепость?","leaveBody":"Другие приложения перестанут видеть приложение {app}, и все разрешения приложения {app} будут отозваны.","leaveBack":"Вернуться можно с помощью текущей фразы.","leaveConfirm":"Покинуть","removeTitle":"Удалить экземпляр {device}?","removeBody":"{device} сразу теряет доступ. Это нельзя отменить.","removeBack":"Любой, у кого ещё есть фраза, может вернуться, пока вы её не смените.","removedTitle":"Сменить фразу сейчас?","removedBody":"{device} больше не в крепости. Но пока фраза прежняя, путь обратно ещё открыт.","removedNote":"Удалены: {devices}. Смените фразу, чтобы закрыть путь обратно.","phraseTitle":"Фраза","leaveRow":"Это приложение в крепости","leaveAbout":"Выводит приложение {app} из крепости. С текущей фразой оно может вернуться.","changedTitle":"Новая фраза","changeElsewhere":"Сменить фразу можно в {apps}.","changeNowhere":"{app} не хранит фразу. Смените её в приложении крепости, которое её показывает.","phraseMoved":"У вашей крепости новая фраза, и {app} не смог перейти на неё сам. Введите новые слова, и он останется тем же участником.","failed":"Не получилось. Попробуйте ещё раз.","statusWhere":"Настройки приложения {app} хранятся в Unraid на устройстве {device}.","statusLoading":"Узнаём, как дела у приложения {app}.","statusQuiet":"Сейчас ничто не требует вашего внимания и ничего не выполняется.","statusNeedsYou":"Требует внимания","statusWork":"Выполняется сейчас","statusNext":"Далее","statusLog":"Последние записи","states":{"off":"Выключено","ok":"Всё в порядке","starting":"Запускается","warning":"Стоит взглянуть","overdue":"Просрочено","failed":"Сбой"},"levels":{"warn":"Предупреждение","error":"Ошибка"}},
    "sk": {"fortress":"Pevnosť","showOthers":"Zobraziť ostatné aplikácie pevnosti","about":"V pevnosti sa tvoje aplikácie navzájom vidia a uľahčujú si prácu. Aplikácia sa do nej dostane pomocou frázy.","othersApps":{"one":"{count} ďalšia aplikácia","few":"{count} ďalšie aplikácie","many":"{count} ďalšej aplikácie","other":"{count} ďalších aplikácií"},"othersDevices":{"one":"{count} zariadení","few":"{count} zariadeniach","many":"{count} zariadenia","other":"{count} zariadeniach"},"othersLine":"{apps} na {devices}. Platí len pre toto zariadenie.","connected":"Pripojené","away":"Nedostupné","viaRelay":"Cez relay","lastSeen":"Naposledy videné {time}","instanceCount":{"one":"{count} inštancia","few":"{count} inštancie","many":"{count} inštancie","other":"{count} inštancií"},"forms":{"container":"Kontajner","plugin":"Plugin pre Unraid","desktop":"Desktopová aplikácia","android":"Aplikácia pre Android","extension":"Rozšírenie prehliadača"},"open":"Otvoriť","remove":"Odstrániť","memberOn":"{app} na {device}","outHint":"Pripoj sa k pevnosti a uvidíš tu svoje ďalšie inštancie aplikácie {app}, a ak chceš, aj ostatné aplikácie.","removedHereTitle":"Odstránené z pevnosti","removedHereHint":"Iná aplikácia v pevnosti odstránila {app}. Zadaj aktuálnu frázu a vrátiš sa späť.","enterPhrase":"Zadať frázu","joinTitle":"Pripojiť sa k pevnosti","word":"Slovo {n}","phraseWhere":"Frázu nájdeš v každej aplikácii pevnosti v časti Inštancie. Vlož ju do prvého poľa a sama sa rozloží do ostatných.","join":"Pripojiť","joinRefused":"Tieto slová neotvárajú žiadnu pevnosť. Skontroluj ich a skús to znova.","joinedTitle":"Aplikácia {app} je teraz v pevnosti","cancel":"Zrušiť","close":"Zavrieť","leaveTitle":"Opustiť pevnosť?","leaveBody":"Ostatné aplikácie už aplikáciu {app} neuvidia a všetky povolenia aplikácie {app} skončia.","leaveBack":"Vrátiť sa môžeš pomocou aktuálnej frázy.","leaveConfirm":"Opustiť","removeTitle":"Odstrániť inštanciu {device}?","removeBody":"{device} hneď stratí prístup. Nedá sa to vrátiť späť.","removeBack":"Kto frázu stále má, dostane sa späť, kým ju nezmeníš.","removedTitle":"Zmeniť frázu hneď teraz?","removedBody":"{device} je vonku. Kým fráza zostane rovnaká, cesta späť je však stále otvorená.","removedNote":"Odstránené: {devices}. Zmeň frázu a cesta späť sa zavrie.","phraseTitle":"Fráza","leaveRow":"Táto aplikácia v pevnosti","leaveAbout":"Odoberie aplikáciu {app} z pevnosti. S aktuálnou frázou sa môže vrátiť.","changedTitle":"Nová fráza","changeElsewhere":"Frázu zmeníš v {apps}.","changeNowhere":"{app} si frázu neuchováva. Zmeň ju v aplikácii pevnosti, ktorá ju ukazuje.","phraseMoved":"Tvoja pevnosť má novú frázu, ku ktorej {app} nedokázal prejsť sám. Zadaj nové slová a zostane rovnakým členom.","failed":"Nepodarilo sa. Skús to, prosím, znova.","statusWhere":"Nastavenia aplikácie {app} nájdeš v systéme Unraid na zariadení {device}.","statusLoading":"Zisťujem, ako sa darí aplikácii {app}.","statusQuiet":"Teraz ťa nič nepotrebuje a nič nebeží.","statusNeedsYou":"Potrebuje ťa","statusWork":"Práve beží","statusNext":"Na rade","statusLog":"Posledné záznamy","states":{"off":"Vypnuté","ok":"Všetko v poriadku","starting":"Spúšťa sa","warning":"Chce to pozrieť","overdue":"Po termíne","failed":"Zlyhalo"},"levels":{"warn":"Varovanie","error":"Chyba"}},
    "sl": {"fortress":"Trdnjava","showOthers":"Pokaži druge aplikacije trdnjave","about":"V trdnjavi se tvoje aplikacije med seboj vidijo in si pomagajo pri delu. Aplikacija pride vanjo s frazo.","othersApps":{"one":"{count} dodatna aplikacija","two":"{count} dodatni aplikaciji","few":"{count} dodatne aplikacije","other":"{count} dodatnih aplikacij"},"othersDevices":{"one":"{count} napravi","two":"{count} napravah","few":"{count} napravah","other":"{count} napravah"},"othersLine":"{apps} na {devices}. Velja samo za to napravo.","connected":"Povezano","away":"Ni dosegljivo","viaRelay":"Prek relaya","lastSeen":"Nazadnje videno {time}","instanceCount":{"one":"{count} instanca","two":"{count} instanci","few":"{count} instance","other":"{count} instanc"},"forms":{"container":"Vsebnik","plugin":"Vtičnik za Unraid","desktop":"Namizna aplikacija","android":"Aplikacija za Android","extension":"Razširitev brskalnika"},"open":"Odpri","remove":"Odstrani","memberOn":"{app} na {device}","outHint":"Pridruži se trdnjavi in tukaj se bodo prikazale tvoje druge instance aplikacije {app}, če želiš, pa tudi ostale aplikacije.","removedHereTitle":"Odstranjeno iz trdnjave","removedHereHint":"Druga aplikacija v trdnjavi je odstranila {app}. Vnesi trenutno frazo, da se vrneš.","enterPhrase":"Vnesi frazo","joinTitle":"Pridruži se trdnjavi","word":"Beseda {n}","phraseWhere":"Vsaka aplikacija v trdnjavi pokaže frazo pod Instance. Prilepi jo v prvo polje in sama se bo razporedila po ostalih.","join":"Pridruži se","joinRefused":"Te besede ne odprejo nobene trdnjave. Preveri jih in poskusi znova.","joinedTitle":"Aplikacija {app} je zdaj v trdnjavi","cancel":"Prekliči","close":"Zapri","leaveTitle":"Želiš zapustiti trdnjavo?","leaveBody":"Druge aplikacije ne bodo več videle aplikacije {app}, vsa dovoljenja aplikacije {app} pa bodo prenehala veljati.","leaveBack":"Vrneš se lahko s trenutno frazo.","leaveConfirm":"Zapusti","removeTitle":"Želiš odstraniti instanco {device}?","removeBody":"{device} takoj izgubi dostop. Tega ni mogoče razveljaviti.","removeBack":"Kdor še ima frazo, se lahko vrne, dokler je ne spremeniš.","removedTitle":"Želiš frazo spremeniti zdaj?","removedBody":"{device} je zunaj. Dokler fraza ostane enaka, je pot nazaj še vedno odprta.","removedNote":"Odstranjeno: {devices}. Spremeni frazo, da zapreš pot nazaj.","phraseTitle":"Fraza","leaveRow":"Ta aplikacija v trdnjavi","leaveAbout":"Odstrani aplikacijo {app} iz trdnjave. S trenutno frazo se lahko vrne.","changedTitle":"Nova fraza","changeElsewhere":"Frazo spremeniš v {apps}.","changeNowhere":"{app} fraze ne hrani. Spremeni jo v aplikaciji trdnjave, ki jo prikazuje.","phraseMoved":"Tvoja trdnjava ima novo frazo, ki ji {app} ni mogel slediti sam. Vnesi nove besede in ostal bo isti član.","failed":"Ni uspelo. Poskusi znova.","statusWhere":"{app} hrani nastavitve v sistemu Unraid na napravi {device}.","statusLoading":"Preverjam, kako gre aplikaciji {app}.","statusQuiet":"Trenutno te nič ne potrebuje in nič ne teče.","statusNeedsYou":"Potrebuje te","statusWork":"Teče zdaj","statusNext":"Sledi","statusLog":"Zadnji vnosi","states":{"off":"Izklopljeno","ok":"Vse v redu","starting":"Se zaganja","warning":"Treba pogledati","overdue":"Zamuja","failed":"Ni uspelo"},"levels":{"warn":"Opozorilo","error":"Napaka"}},
    "sr": {"fortress":"Тврђава","showOthers":"Прикажи остале апликације тврђаве","about":"У тврђави се твоје апликације међусобно виде и помажу једна другој у раду. Апликација улази у њу помоћу фразе.","othersApps":{"one":"{count} додатна апликација","few":"{count} додатне апликације","other":"{count} додатних апликација"},"othersDevices":{"one":"{count} уређају","few":"{count} уређаја","other":"{count} уређаја"},"othersLine":"{apps} на {devices}. Важи само за овај уређај.","connected":"Повезано","away":"Недоступно","viaRelay":"Преко релеја","lastSeen":"Последњи пут виђено {time}","instanceCount":{"one":"{count} инстанца","few":"{count} инстанце","other":"{count} инстанци"},"forms":{"container":"Контејнер","plugin":"Unraid додатак","desktop":"Апликација за рачунар","android":"Апликација за Android","extension":"Проширење прегледача"},"open":"Отвори","remove":"Уклони","memberOn":"{app} на {device}","outHint":"Придружи се тврђави и овде ће се приказати твоје остале инстанце апликације {app}, а по жељи и остале апликације.","removedHereTitle":"Уклоњено из тврђаве","removedHereHint":"Друга апликација у тврђави је уклонила {app}. Унеси тренутну фразу да се вратиш.","enterPhrase":"Унеси фразу","joinTitle":"Придружи се тврђави","word":"Реч {n}","phraseWhere":"Свака апликација у тврђави приказује фразу под Инстанце. Налепи је у прво поље и сама ће се распоредити по осталима.","join":"Придружи се","joinRefused":"Ове речи не отварају ниједну тврђаву. Провери их и покушај поново.","joinedTitle":"Апликација {app} је сада у тврђави","cancel":"Откажи","close":"Затвори","leaveTitle":"Напустити тврђаву?","leaveBody":"Остале апликације више неће видети апликацију {app}, а све дозволе апликације {app} престају да важе.","leaveBack":"Можеш да се вратиш помоћу тренутне фразе.","leaveConfirm":"Напусти","removeTitle":"Уклонити инстанцу {device}?","removeBody":"{device} одмах губи приступ. Ово не може да се поништи.","removeBack":"Ко још има фразу, може да се врати док је не промениш.","removedTitle":"Променити фразу одмах?","removedBody":"{device} је напољу. Док фраза остаје иста, пут назад је и даље отворен.","removedNote":"Уклоњено: {devices}. Промени фразу да затвориш пут назад.","phraseTitle":"Фраза","leaveRow":"Ова апликација у тврђави","leaveAbout":"Изводи апликацију {app} из тврђаве. Уз тренутну фразу може да се врати.","changedTitle":"Нова фраза","changeElsewhere":"Фразу мењаш у {apps}.","changeNowhere":"{app} не чува фразу. Промени је у апликацији тврђаве која је приказује.","phraseMoved":"Твоја тврђава има нову фразу коју {app} није могао сам да прати. Унеси нове речи и остаће исти члан.","failed":"Није успело. Покушај поново.","statusWhere":"{app} држи подешавања у систему Unraid на уређају {device}.","statusLoading":"Проверавам како је {app}.","statusQuiet":"Тренутно те ништа не чека и ништа се не извршава.","statusNeedsYou":"Чека те","statusWork":"Ради сада","statusNext":"Следи","statusLog":"Последњи записи","states":{"off":"Искључено","ok":"Све у реду","starting":"Покреће се","warning":"Треба погледати","overdue":"Касни","failed":"Није успело"},"levels":{"warn":"Упозорење","error":"Грешка"}},
    "sv": {"fortress":"Fästning","showOthers":"Visa fästningens andra appar","about":"I en fästning ser dina appar varandra och delar på arbetet. En app kommer in med frasen.","othersApps":{"one":"{count} annan app","other":"{count} andra appar"},"othersDevices":{"one":"{count} enhet","other":"{count} enheter"},"othersLine":"{apps} på {devices}. Gäller bara den här enheten.","connected":"Ansluten","away":"Går inte att nå","viaRelay":"Via relä","lastSeen":"Senast sedd {time}","instanceCount":{"one":"{count} instans","other":"{count} instanser"},"forms":{"container":"Container","plugin":"Unraid-plugin","desktop":"Skrivbordsapp","android":"Android-app","extension":"Webbläsartillägg"},"open":"Öppna","remove":"Ta bort","memberOn":"{app} på {device}","outHint":"Gå med i en fästning så visas dina andra instanser av {app} här, och de andra apparna också om du vill.","removedHereTitle":"Borttagen från fästningen","removedHereHint":"En annan app i fästningen har tagit bort {app}. Ange den aktuella frasen för att komma in igen.","enterPhrase":"Ange fras","joinTitle":"Gå med i en fästning","word":"Ord {n}","phraseWhere":"Alla appar i fästningen visar frasen under Instanser. Klistra in den i första fältet så fördelas den över resten.","join":"Gå med","joinRefused":"De här orden öppnar ingen fästning. Kontrollera dem och försök igen.","joinedTitle":"{app} är nu med i fästningen","cancel":"Avbryt","close":"Stäng","leaveTitle":"Lämna fästningen?","leaveBody":"De andra apparna ser inte längre {app}, och alla behörigheter med {app} upphör.","leaveBack":"Du kommer tillbaka med den aktuella frasen.","leaveConfirm":"Lämna","removeTitle":"Ta bort {device}?","removeBody":"{device} förlorar sin åtkomst direkt. Det går inte att ångra.","removeBack":"Den som fortfarande har frasen kan komma in igen tills du byter den.","removedTitle":"Byta frasen nu?","removedBody":"{device} är ute. Så länge frasen är densamma står vägen tillbaka ändå öppen.","removedNote":"Borttagna: {devices}. Byt frasen för att stänga vägen tillbaka.","phraseTitle":"Fras","leaveRow":"Den här appen i fästningen","leaveAbout":"Tar ut {app} ur fästningen. Med den aktuella frasen kommer den tillbaka.","changedTitle":"Ny fras","changeElsewhere":"Frasen byter du i {apps}.","changeNowhere":"{app} sparar inte frasen. Byt den i en app i fästningen som visar den.","phraseMoved":"Din fästning har en ny fras som {app} inte kunde följa på egen hand. Ange de nya orden så fortsätter den som samma medlem.","failed":"Det gick inte. Försök igen.","statusWhere":"{app} har sina inställningar i Unraid på {device}.","statusLoading":"Frågar {app} hur det går.","statusQuiet":"Inget behöver dig just nu, och inget körs.","statusNeedsYou":"Behöver dig","statusWork":"Körs nu","statusNext":"Härnäst","statusLog":"Senaste posterna","states":{"off":"Av","ok":"Allt bra","starting":"Startar","warning":"Behöver en titt","overdue":"Försenad","failed":"Misslyckades"},"levels":{"warn":"Varning","error":"Fel"}},
    "th": {"fortress":"ป้อมปราการ","showOthers":"แสดงแอปอื่นในป้อมปราการ","about":"ในป้อมปราการ แอปของคุณจะมองเห็นกันและช่วยแบ่งงานกันทำ แอปเข้าร่วมได้ด้วยวลี","othersApps":{"other":"แอปอื่นอีก {count} แอป"},"othersDevices":{"other":"{count} อุปกรณ์"},"othersLine":"{apps} บน {devices} มีผลกับอุปกรณ์นี้เท่านั้น","connected":"เชื่อมต่อแล้ว","away":"เข้าถึงไม่ได้","viaRelay":"ผ่านรีเลย์","lastSeen":"เห็นล่าสุด {time}","instanceCount":{"other":"{count} อินสแตนซ์"},"forms":{"container":"คอนเทนเนอร์","plugin":"ปลั๊กอิน Unraid","desktop":"แอปเดสก์ท็อป","android":"แอป Android","extension":"ส่วนขยายเบราว์เซอร์"},"open":"เปิด","remove":"นำออก","memberOn":"{app} บน {device}","outHint":"เมื่อเข้าร่วมป้อมปราการ อินสแตนซ์อื่นของ {app} จะแสดงที่นี่ และแสดงแอปอื่นด้วยก็ได้ถ้าต้องการ","removedHereTitle":"ถูกนำออกจากป้อมปราการ","removedHereHint":"แอปอื่นในป้อมปราการนำ {app} ออกแล้ว ใส่วลีปัจจุบันเพื่อกลับเข้ามา","enterPhrase":"ป้อนวลี","joinTitle":"เข้าร่วมป้อมปราการ","word":"คำที่ {n}","phraseWhere":"ทุกแอปในป้อมปราการจะแสดงวลีไว้ใต้อินสแตนซ์ วางวลีลงในช่องแรก แล้วคำที่เหลือจะกระจายไปยังช่องอื่นเอง","join":"เข้าร่วม","joinRefused":"คำเหล่านี้เปิดป้อมปราการไม่ได้ โปรดตรวจสอบแล้วลองอีกครั้ง","joinedTitle":"{app} อยู่ในป้อมปราการแล้ว","cancel":"ยกเลิก","close":"ปิด","leaveTitle":"ออกจากป้อมปราการไหม","leaveBody":"แอปอื่นจะมองไม่เห็น {app} อีก และสิทธิ์ทั้งหมดที่เกี่ยวกับ {app} จะสิ้นสุดลง","leaveBack":"คุณกลับมาได้ด้วยวลีปัจจุบัน","leaveConfirm":"ออก","removeTitle":"นำ {device} ออกไหม","removeBody":"{device} จะเข้าถึงไม่ได้ทันที และย้อนกลับไม่ได้","removeBack":"ใครที่ยังมีวลีอยู่จะกลับเข้ามาได้ จนกว่าคุณจะเปลี่ยนวลี","removedTitle":"เปลี่ยนวลีตอนนี้ไหม","removedBody":"{device} ออกไปแล้ว แต่ตราบใดที่วลียังเหมือนเดิม ทางกลับเข้ามาก็ยังเปิดอยู่","removedNote":"นำออกแล้ว: {devices} เปลี่ยนวลีเพื่อปิดทางกลับเข้ามา","phraseTitle":"วลี","leaveRow":"แอปนี้ในป้อมปราการ","leaveAbout":"นำ {app} ออกจากป้อมปราการ แอปนี้กลับมาได้ด้วยวลีปัจจุบัน","changedTitle":"วลีใหม่","changeElsewhere":"คุณเปลี่ยนวลีได้ที่ {apps}","changeNowhere":"{app} ไม่เก็บวลีไว้ เปลี่ยนวลีได้ในแอปของป้อมปราการที่แสดงวลี","phraseMoved":"ป้อมปราการของคุณมีวลีใหม่ที่ {app} ตามไปเองไม่ได้ ใส่คำใหม่ แล้วจะยังเป็นสมาชิกคนเดิม","failed":"ไม่สำเร็จ โปรดลองอีกครั้ง","statusWhere":"{app} เก็บการตั้งค่าไว้ใน Unraid บน {device}","statusLoading":"กำลังถามสถานะจาก {app}","statusQuiet":"ตอนนี้ไม่มีอะไรต้องให้คุณจัดการ และไม่มีอะไรกำลังทำงานอยู่","statusNeedsYou":"ต้องให้คุณดู","statusWork":"กำลังทำงาน","statusNext":"ถัดไป","statusLog":"รายการล่าสุด","states":{"off":"ปิด","ok":"เรียบร้อยดี","starting":"กำลังเริ่ม","warning":"ควรตรวจดู","overdue":"เลยกำหนด","failed":"ล้มเหลว"},"levels":{"warn":"คำเตือน","error":"ข้อผิดพลาด"}},
    "tr": {"fortress":"Kale","showOthers":"Kaledeki diğer uygulamaları göster","about":"Bir kalede uygulamaların birbirini görür ve birbirinin yükünü hafifletir. Bir uygulama kaleye ifadeyle girer.","othersApps":{"one":"{count} uygulama daha","other":"{count} uygulama daha"},"othersDevices":{"one":"{count} cihazda","other":"{count} cihazda"},"othersLine":"{devices} {apps}. Yalnızca bu cihaz için geçerli.","connected":"Bağlı","away":"Ulaşılamıyor","viaRelay":"Röle üzerinden","lastSeen":"Son görülme: {time}","instanceCount":{"one":"{count} örnek","other":"{count} örnek"},"forms":{"container":"Konteyner","plugin":"Unraid eklentisi","desktop":"Masaüstü uygulaması","android":"Android uygulaması","extension":"Tarayıcı uzantısı"},"open":"Aç","remove":"Kaldır","memberOn":"{device} üzerinde {app}","outHint":"Bir kaleye katıldığında {app} uygulamasının diğer örnekleri burada görünür. İstersen diğer uygulamalar da.","removedHereTitle":"Kaleden kaldırıldı","removedHereHint":"Kaledeki başka bir uygulama {app} uygulamasını kaldırdı. Geri girmek için güncel ifadeyi gir.","enterPhrase":"İfadeyi gir","joinTitle":"Bir kaleye katıl","word":"{n}. kelime","phraseWhere":"Kaledeki her uygulama ifadeyi Örnekler bölümünde gösterir. İlk alana yapıştırdığında kalan alanlara kendiliğinden dağılır.","join":"Katıl","joinRefused":"Bu kelimeler hiçbir kaleyi açmıyor. Kontrol edip yeniden dene.","joinedTitle":"{app} uygulaması artık kalede","cancel":"İptal","close":"Kapat","leaveTitle":"Kaleden ayrılmak istiyor musun?","leaveBody":"Diğer uygulamalar {app} uygulamasını artık görmez ve {app} ile olan her izin sona erer.","leaveBack":"Güncel ifadeyle geri dönebilirsin.","leaveConfirm":"Ayrıl","removeTitle":"{device} kaldırılsın mı?","removeBody":"{device} erişimini hemen kaybeder. Bu geri alınamaz.","removeBack":"İfade hâlâ elinde olan herkes, sen onu değiştirene kadar geri girebilir.","removedTitle":"İfade şimdi değiştirilsin mi?","removedBody":"{device} artık dışarıda. Ama ifade aynı kaldıkça geri dönüş yolu hâlâ açık.","removedNote":"Kaldırılanlar: {devices}. Geri dönüş yolunu kapatmak için ifadeyi değiştir.","phraseTitle":"İfade","leaveRow":"Kaledeki bu uygulama","leaveAbout":"{app} uygulamasını kaleden çıkarır. Güncel ifadeyle geri dönebilir.","changedTitle":"Yeni ifade","changeElsewhere":"İfadeyi {apps} içinde değiştirirsin.","changeNowhere":"{app} ifadeyi saklamaz. İfadeyi kalede onu gösteren bir uygulamada değiştir.","phraseMoved":"Kalenin yeni bir ifadesi var ve {app} buna kendi başına geçemedi. Yeni kelimeleri gir, aynı üye olarak devam etsin.","failed":"Olmadı. Lütfen yeniden dene.","statusWhere":"{app} ayarlarını {device} üzerindeki Unraid'de tutar.","statusLoading":"{app} uygulamasının durumu soruluyor.","statusQuiet":"Şu an seni bekleyen bir şey yok ve çalışan bir şey de yok.","statusNeedsYou":"Seni bekliyor","statusWork":"Şu an çalışıyor","statusNext":"Sıradaki","statusLog":"Son kayıtlar","states":{"off":"Kapalı","ok":"Her şey yolunda","starting":"Başlıyor","warning":"Bakılmalı","overdue":"Gecikti","failed":"Başarısız"},"levels":{"warn":"Uyarı","error":"Hata"}},
    "uk": {"fortress":"Фортеця","showOthers":"Показати інші застосунки фортеці","about":"У фортеці ваші застосунки бачать одне одного й допомагають одне одному з роботою. Застосунок потрапляє до неї за допомогою фрази.","othersApps":{"one":"{count} інший застосунок","few":"{count} інші застосунки","many":"{count} інших застосунків","other":"{count} іншого застосунку"},"othersDevices":{"one":"{count} пристрої","few":"{count} пристроях","many":"{count} пристроях","other":"{count} пристроях"},"othersLine":"{apps} на {devices}. Діє лише на цьому пристрої.","connected":"Підключено","away":"Недоступно","viaRelay":"Через реле","lastSeen":"Востаннє в мережі {time}","instanceCount":{"one":"{count} екземпляр","few":"{count} екземпляри","many":"{count} екземплярів","other":"{count} екземпляра"},"forms":{"container":"Контейнер","plugin":"Плагін Unraid","desktop":"Застосунок для комп’ютера","android":"Застосунок для Android","extension":"Розширення браузера"},"open":"Відкрити","remove":"Видалити","memberOn":"{app} на {device}","outHint":"Приєднайтеся до фортеці, і тут з’являться інші ваші екземпляри застосунку {app}, а за бажання й решта застосунків.","removedHereTitle":"Видалено з фортеці","removedHereHint":"Інший застосунок фортеці видалив {app}. Введіть поточну фразу, щоб повернутися.","enterPhrase":"Ввести фразу","joinTitle":"Приєднатися до фортеці","word":"Слово {n}","phraseWhere":"Фразу показує кожен застосунок фортеці в розділі Екземпляри. Вставте її в перше поле, і вона сама розподілиться по решті.","join":"Приєднатися","joinRefused":"Ці слова не відкривають жодної фортеці. Перевірте їх і спробуйте ще раз.","joinedTitle":"Застосунок {app} тепер у фортеці","cancel":"Скасувати","close":"Закрити","leaveTitle":"Вийти з фортеці?","leaveBody":"Інші застосунки більше не бачитимуть застосунок {app}, і всі дозволи застосунку {app} буде скасовано.","leaveBack":"Повернутися можна з поточною фразою.","leaveConfirm":"Вийти","removeTitle":"Видалити екземпляр {device}?","removeBody":"{device} одразу втрачає доступ. Це не можна скасувати.","removeBack":"Будь-хто, у кого ще є фраза, може повернутися, доки ви її не зміните.","removedTitle":"Змінити фразу зараз?","removedBody":"{device} уже поза фортецею. Але поки фраза та сама, шлях назад ще відкритий.","removedNote":"Видалено: {devices}. Змініть фразу, щоб закрити шлях назад.","phraseTitle":"Фраза","leaveRow":"Цей застосунок у фортеці","leaveAbout":"Виводить застосунок {app} з фортеці. З поточною фразою він може повернутися.","changedTitle":"Нова фраза","changeElsewhere":"Змінити фразу можна в {apps}.","changeNowhere":"{app} не зберігає фразу. Змініть її в застосунку фортеці, який її показує.","phraseMoved":"У вашої фортеці нова фраза, і {app} не зміг перейти на неї сам. Введіть нові слова, і він залишиться тим самим учасником.","failed":"Не вдалося. Спробуйте ще раз.","statusWhere":"Налаштування застосунку {app} зберігаються в Unraid на пристрої {device}.","statusLoading":"Дізнаємося, як справи в застосунку {app}.","statusQuiet":"Зараз ніщо не потребує вашої уваги і нічого не виконується.","statusNeedsYou":"Потребує уваги","statusWork":"Виконується зараз","statusNext":"Далі","statusLog":"Останні записи","states":{"off":"Вимкнено","ok":"Усе гаразд","starting":"Запускається","warning":"Варто глянути","overdue":"Прострочено","failed":"Збій"},"levels":{"warn":"Попередження","error":"Помилка"}},
    "vi": {"fortress":"Pháo đài","showOthers":"Hiện các ứng dụng khác trong pháo đài","about":"Trong pháo đài, các ứng dụng của bạn thấy nhau và san sẻ công việc cho nhau. Ứng dụng vào pháo đài bằng cụm từ.","othersApps":{"other":"{count} ứng dụng khác"},"othersDevices":{"other":"{count} thiết bị"},"othersLine":"{apps} trên {devices}. Chỉ áp dụng cho thiết bị này.","connected":"Đã kết nối","away":"Không truy cập được","viaRelay":"Qua relay","lastSeen":"Thấy lần cuối {time}","instanceCount":{"other":"{count} phiên bản"},"forms":{"container":"Container","plugin":"Plugin Unraid","desktop":"Ứng dụng máy tính","android":"Ứng dụng Android","extension":"Tiện ích trình duyệt"},"open":"Mở","remove":"Xóa","memberOn":"{app} trên {device}","outHint":"Tham gia một pháo đài để các phiên bản khác của {app} hiện ở đây, kèm các ứng dụng khác nếu bạn muốn.","removedHereTitle":"Đã bị xóa khỏi pháo đài","removedHereHint":"Một ứng dụng khác trong pháo đài đã xóa {app}. Nhập cụm từ hiện tại để vào lại.","enterPhrase":"Nhập cụm từ","joinTitle":"Tham gia pháo đài","word":"Từ {n}","phraseWhere":"Mọi ứng dụng trong pháo đài đều hiện cụm từ ở mục Phiên bản. Dán vào ô đầu tiên, cụm từ sẽ tự chia sang các ô còn lại.","join":"Tham gia","joinRefused":"Các từ này không mở được pháo đài nào. Hãy kiểm tra rồi thử lại.","joinedTitle":"{app} đã vào pháo đài","cancel":"Hủy","close":"Đóng","leaveTitle":"Rời pháo đài?","leaveBody":"Các ứng dụng khác sẽ không thấy {app} nữa, và mọi quyền liên quan đến {app} đều kết thúc.","leaveBack":"Bạn có thể quay lại bằng cụm từ hiện tại.","leaveConfirm":"Rời đi","removeTitle":"Xóa {device}?","removeBody":"{device} mất quyền truy cập ngay lập tức. Không thể hoàn tác.","removeBack":"Ai còn giữ cụm từ vẫn có thể vào lại cho đến khi bạn đổi nó.","removedTitle":"Đổi cụm từ ngay bây giờ?","removedBody":"{device} đã ra ngoài. Nhưng khi cụm từ vẫn như cũ, đường quay lại vẫn còn mở.","removedNote":"Đã xóa: {devices}. Đổi cụm từ để đóng đường quay lại.","phraseTitle":"Cụm từ","leaveRow":"Ứng dụng này trong pháo đài","leaveAbout":"Đưa {app} ra khỏi pháo đài. Với cụm từ hiện tại, ứng dụng có thể quay lại.","changedTitle":"Cụm từ mới","changeElsewhere":"Bạn đổi cụm từ trong {apps}.","changeNowhere":"{app} không giữ cụm từ. Hãy đổi cụm từ trong một ứng dụng của pháo đài có hiện nó.","phraseMoved":"Pháo đài của bạn có cụm từ mới mà {app} không tự theo kịp. Nhập các từ mới để nó vẫn là thành viên như trước.","failed":"Chưa thành công. Vui lòng thử lại.","statusWhere":"{app} lưu cài đặt trong Unraid trên {device}.","statusLoading":"Đang hỏi thăm {app}.","statusQuiet":"Hiện chưa có gì cần bạn, và cũng không có gì đang chạy.","statusNeedsYou":"Cần bạn","statusWork":"Đang chạy","statusNext":"Sắp tới","statusLog":"Mục mới nhất","states":{"off":"Tắt","ok":"Mọi thứ ổn","starting":"Đang khởi động","warning":"Cần xem qua","overdue":"Quá hạn","failed":"Thất bại"},"levels":{"warn":"Cảnh báo","error":"Lỗi"}},
    "zh": {"fortress":"堡垒","showOthers":"显示堡垒中的其他应用","about":"在堡垒中，你的应用能互相看到，并彼此分担工作。应用凭口令加入。","othersApps":{"other":"另外 {count} 个应用"},"othersDevices":{"other":"{count} 台设备"},"othersLine":"{devices}上的{apps}。仅对此设备生效。","connected":"已连接","away":"无法连接","viaRelay":"通过中继","lastSeen":"上次在线 {time}","instanceCount":{"other":"{count} 个实例"},"forms":{"container":"容器","plugin":"Unraid 插件","desktop":"桌面应用","android":"Android 应用","extension":"浏览器扩展"},"open":"打开","remove":"移除","memberOn":"{device} 上的 {app}","outHint":"加入堡垒后，{app} 的其他实例会显示在这里，你也可以选择同时显示其他应用。","removedHereTitle":"已被移出堡垒","removedHereHint":"堡垒中的另一个应用移除了 {app}。输入当前口令即可重新加入。","enterPhrase":"输入口令","joinTitle":"加入堡垒","word":"第 {n} 个词","phraseWhere":"堡垒中的每个应用都会在“实例”下显示口令。粘贴到第一个输入框，其余输入框会自动填好。","join":"加入","joinRefused":"这些词无法打开任何堡垒。请检查后重试。","joinedTitle":"{app} 已加入堡垒","cancel":"取消","close":"关闭","leaveTitle":"要退出堡垒吗？","leaveBody":"其他应用将不再看到 {app}，与 {app} 相关的所有许可也会终止。","leaveBack":"凭当前口令可以回来。","leaveConfirm":"退出","removeTitle":"要移除 {device} 吗？","removeBody":"{device} 会立即失去访问权限，且无法撤销。","removeBack":"在你更改口令之前，知道口令的人都能重新加入。","removedTitle":"现在更改口令吗？","removedBody":"{device} 已被移出。但只要口令不变，回来的路就仍然开着。","removedNote":"已移除：{devices}。更改口令即可关上回来的路。","phraseTitle":"口令","leaveRow":"堡垒中的这个应用","leaveAbout":"将 {app} 移出堡垒。凭当前口令可以回来。","changedTitle":"新口令","changeElsewhere":"请在 {apps} 中更改口令。","changeNowhere":"{app} 不保存口令。请在堡垒中能显示口令的应用里更改。","phraseMoved":"你的堡垒换了新口令，{app} 没能自己跟上。输入新的单词，它仍是原来的成员。","failed":"操作未成功，请重试。","statusWhere":"{app} 的设置保存在 {device} 的 Unraid 中。","statusLoading":"正在询问 {app} 的状态。","statusQuiet":"目前没有需要你处理的事，也没有正在运行的任务。","statusNeedsYou":"需要你处理","statusWork":"正在运行","statusNext":"即将运行","statusLog":"最新记录","states":{"off":"关闭","ok":"一切正常","starting":"正在启动","warning":"需要看一下","overdue":"已逾期","failed":"失败"},"levels":{"warn":"警告","error":"错误"}},
  };
  const GLYPHS = {
    IconInfo: "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 14 14\" width=\"16\" height=\"16\" fill=\"currentColor\"><path fill=\"currentColor\" fill-rule=\"evenodd\" d=\"M7 14c3.866 0 7 -3.134 7 -7 0 -3.86599 -3.134 -7 -7 -7 -3.86599 0 -7 3.13401 -7 7 0 3.866 3.13401 7 7 7ZM5.5 9.375c-0.34518 0 -0.625 0.27982 -0.625 0.625 0 0.3452 0.27982 0.625 0.625 0.625h3c0.34518 0 0.625 -0.2798 0.625 -0.625 0 -0.34518 -0.27982 -0.625 -0.625 -0.625h-0.875V6.5c0 -0.34518 -0.27982 -0.625 -0.625 -0.625H6c-0.34518 0 -0.625 0.27982 -0.625 0.625s0.27982 0.625 0.625 0.625h0.375v2.25H5.5ZM8 4c0 0.55228 -0.44772 1 -1 1s-1 -0.44772 -1 -1 0.44772 -1 1 -1 1 0.44772 1 1Z\" clip-rule=\"evenodd\" stroke-width=\"1\"></path></svg>",
    IconSignIn: "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 14 14\" width=\"16\" height=\"16\" fill=\"currentColor\"><path fill=\"currentColor\" fill-rule=\"evenodd\" d=\"M14 12.5C14 13.328427 13.328427 14 12.5 14h-7c-0.82843 0 -1.5 -0.671573 -1.5 -1.5v-3.24998h0.51563c0.0888 0.70641 0.54957 1.32045 1.219 1.59774c0.74735 0.30956 1.60759 0.13845 2.17958 -0.43354l2 -2c0.78105 -0.78105 0.78105 -2.04738 0 -2.82843l-2 -1.99999c-0.57199 -0.572 -1.43223 -0.7431 -2.17958 -0.4336c-0.66944 0.2773 -1.13022 0.8914 -1.219 1.59782H4V1.5c0 -0.8284 0.67157 -1.5 1.5 -1.5h7C13.328427 0 14 0.6716 14 1.5v11Zm-7.78701 -2.80711c-0.28026 -0.11609 -0.46299 -0.38957 -0.46299 -0.69291v-1H1c-0.5523 0 -1 -0.44772 -1 -1c0 -0.55229 0.4477 -1 1 -1H5.75v-1c0 -0.30335 0.18273 -0.57683 0.46299 -0.69291c0.28025 -0.11609 0.60284 -0.05192 0.81734 0.16258l2 2c0.29289 0.29289 0.29289 0.76777 0 1.06066l-2 2c-0.2145 0.2145 -0.53709 0.27866 -0.81734 0.16258Z\" clip-rule=\"evenodd\" stroke-width=\"1\"></path></svg>",
    IconSignOut: "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 14 14\" width=\"16\" height=\"16\" fill=\"currentColor\"><path fill=\"currentColor\" fill-rule=\"evenodd\" d=\"M0 1.5C0 0.671573 0.671573 0 1.5 0h7c0.82843 0 1.5 0.671573 1.5 1.5v1.93863c-0.40264 0.32239 -0.66907 0.7919 -0.73437 1.31135H5.75c-1.24264 0 -2.25 1.00735 -2.25 2.25 0 1.24264 1.00736 2.25 2.25 2.25h3.51563c0.06528 0.51946 0.33172 0.98902 0.73437 1.31142V12.5c0 0.8284 -0.67157 1.5 -1.5 1.5h-7C0.671573 14 0 13.3284 0 12.5v-11Zm10.963 2.80711c-0.2803 0.11609 -0.463 0.38957 -0.463 0.69291v1H5.75c-0.55228 0 -1 0.44772 -1 1 0 0.55229 0.44772 1 1 1h4.75v1c0 0.30335 0.1827 0.57683 0.463 0.69291 0.2802 0.11609 0.6028 0.05192 0.8173 -0.16258l2 -2c0.2929 -0.29289 0.2929 -0.76777 0 -1.06066l-2 -2c-0.2145 -0.2145 -0.5371 -0.27866 -0.8173 -0.16258Z\" clip-rule=\"evenodd\" stroke-width=\"1\"></path></svg>",
    IconTrash: "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 14 14\" width=\"16\" height=\"16\" fill=\"currentColor\"><g><path fill=\"currentColor\" fill-rule=\"evenodd\" d=\"M5.76256 2.01256C6.09075 1.68437 6.53587 1.5 7 1.5c0.46413 0 0.90925 0.18437 1.23744 0.51256 0.20736 0.20737 0.35731 0.46141 0.43961 0.73744h-3.3541c0.0823 -0.27603 0.23225 -0.53007 0.43961 -0.73744ZM3.78868 2.75c0.10537 -0.67679 0.42285 -1.30773 0.91322 -1.798097C5.3114 0.34241 6.13805 0 7 0c0.86195 0 1.6886 0.34241 2.2981 0.951903 0.49037 0.490367 0.8079 1.121307 0.9132 1.798097H13c0.4142 0 0.75 0.33579 0.75 0.75 0 0.41422 -0.3358 0.75 -0.75 0.75h-1v8.25c0 0.3978 -0.158 0.7794 -0.4393 1.0607S10.8978 14 10.5 14h-7c-0.39783 0 -0.77936 -0.158 -1.06066 -0.4393C2.15804 13.2794 2 12.8978 2 12.5V4.25H1c-0.414214 0 -0.75 -0.33578 -0.75 -0.75 0 -0.41421 0.335786 -0.75 0.75 -0.75h2.78868ZM5 5.87646c0.34518 0 0.625 0.27983 0.625 0.625V10.503c0 0.3451 -0.27982 0.625 -0.625 0.625s-0.625 -0.2799 -0.625 -0.625V6.50146c0 -0.34517 0.27982 -0.625 0.625 -0.625Zm4.625 0.625c0 -0.34517 -0.27982 -0.625 -0.625 -0.625s-0.625 0.27983 -0.625 0.625V10.503c0 0.3451 0.27982 0.625 0.625 0.625s0.625 -0.2799 0.625 -0.625V6.50146Z\" clip-rule=\"evenodd\" stroke-width=\"1\"></path></g></svg>",
    IconClose: "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 14 14\" width=\"16\" height=\"16\" fill=\"currentColor\"><path fill-rule=\"evenodd\" d=\"M11.957 13.716 L12.056 13.789 L12.272 13.905 L12.506 13.976 L12.75 14 L12.994 13.976 L13.228 13.905 L13.444 13.789 L13.543 13.716 L13.716 13.543 L13.789 13.444 L13.905 13.228 L13.976 12.994 L14 12.75 L13.976 12.506 L13.905 12.272 L13.852 12.161 L13.789 12.056 L13.634 11.866 L8.768 7 L13.716 2.043 L13.789 1.944 L13.905 1.728 L13.976 1.494 L14 1.25 L13.976 1.006 L13.905 0.772 L13.789 0.556 L13.716 0.457 L13.543 0.284 L13.339 0.148 L13.113 0.054 L12.873 0.006 L12.627 0.006 L12.387 0.054 L12.272 0.095 L12.056 0.211 L11.866 0.366 L7 5.232 L2.043 0.284 L1.839 0.148 L1.613 0.054 L1.494 0.024 L1.25 0 L1.127 0.006 L0.887 0.054 L0.661 0.148 L0.457 0.284 L0.284 0.457 L0.211 0.556 L0.095 0.772 L0.024 1.006 L0 1.25 L0.024 1.494 L0.095 1.728 L0.211 1.944 L0.366 2.134 L5.232 7 L0.284 11.957 L0.211 12.056 L0.095 12.272 L0.024 12.506 L0 12.75 L0.024 12.994 L0.095 13.228 L0.211 13.444 L0.284 13.543 L0.457 13.716 L0.661 13.852 L0.772 13.905 L1.006 13.976 L1.25 14 L1.494 13.976 L1.728 13.905 L1.944 13.789 L2.134 13.634 L7 8.768Z\" /></svg>",
    IconCheck: "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0.3 0.549 13.399 13.399\" width=\"16\" height=\"16\" fill=\"currentColor\"><path fill-rule=\"evenodd\" d=\"M4.381 12.219 L4.574 12.378 L4.793 12.497 L5.032 12.572 L5.155 12.592 L5.405 12.596 L5.651 12.552 L5.884 12.461 L6.096 12.328 L6.278 12.157 L13.457 3.957 L13.584 3.736 L13.666 3.495 L13.699 3.242 L13.683 2.988 L13.656 2.863 L13.566 2.625 L13.431 2.408 L13.257 2.222 L13.05 2.074 L12.936 2.016 L12.695 1.934 L12.442 1.901 L12.188 1.917 L11.942 1.983 L11.713 2.096 L11.511 2.251 L5.238 9.399 L2.519 6.681 L2.425 6.595 L2.213 6.454 L1.977 6.356 L1.854 6.325 L1.6 6.3 L1.346 6.325 L1.223 6.356 L0.987 6.454 L0.775 6.595 L0.595 6.775 L0.454 6.987 L0.399 7.103 L0.325 7.346 L0.3 7.6 L0.306 7.727 L0.356 7.977 L0.399 8.097 L0.519 8.322 L0.681 8.519Z\" /></svg>",
    IconExternalLink: "<svg xmlns=\"http://www.w3.org/2000/svg\" viewBox=\"0 0 14 14\" width=\"16\" height=\"16\" fill=\"currentColor\"><g><path fill=\"currentColor\" fill-rule=\"evenodd\" d=\"M7 2 2.5 2a0.5 0.5 0 0 0 -0.5 0.5l0 9a0.5 0.5 0 0 0 0.5 0.5l9 0a0.5 0.5 0 0 0 0.5 -0.5L12 7a1 1 0 1 1 2 0l0 4.5a2.5 2.5 0 0 1 -2.5 2.5l-9 0A2.5 2.5 0 0 1 0 11.5l0 -9A2.5 2.5 0 0 1 2.5 0L7 0a1 1 0 0 1 0 2ZM9.538 0.309A0.5 0.5 0 0 1 10 0l3.5 0a0.5 0.5 0 0 1 0.5 0.5L14 4a0.5 0.5 0 0 1 -0.854 0.354L12.104 3.31 7.707 7.707a1 1 0 0 1 -1.414 -1.414l4.396 -4.397L9.646 0.854a0.5 0.5 0 0 1 -0.108 -0.545Z\" clip-rule=\"evenodd\" stroke-width=\"1\"></path></g></svg>",
  };
  /* texts:end */

  const POLL_MS = 20000;
  const PHRASE_LENGTH = 12;
  const STATUS_LOG_LINES = 10;

  function textFor(lang) {
    const tag = String(lang || "en").toLowerCase();
    return TEXTS[tag] || TEXTS[tag.split(/[-_]/)[0]] || TEXTS.en;
  }

  function fill(template, values) {
    return String(template).replace(/\{(\w+)\}/g, (all, name) => (name in values ? String(values[name]) : all));
  }

  function plural(lang, forms, count) {
    let category = "other";
    try {
      category = new Intl.PluralRules(lang).select(count);
    } catch (e) {
      // An unknown tag keeps the general form.
    }
    return fill(forms[category] || forms.other, { count: count.toLocaleString(lang) });
  }

  function listOf(lang, items, type) {
    try {
      return new Intl.ListFormat(lang, { type: type || "conjunction" }).format(items);
    } catch (e) {
      return items.join(", ");
    }
  }

  function phraseFromText(text) {
    return String(text)
      .toLowerCase()
      .split(/[\s,;]+/)
      .filter((w) => w !== "" && !/^\d+[.):]?$/.test(w));
  }

  // One word stays as typed, a word and a space moves on, several words
  // spread over the fields from this one on.
  function enterWords(words, at, typed) {
    const next = words.slice();
    if (!/\s/.test(typed)) {
      next[at] = typed;
      return { words: next, focus: at };
    }
    const found = phraseFromText(typed).slice(0, PHRASE_LENGTH - at);
    if (!found.length) {
      next[at] = "";
      return { words: next, focus: at };
    }
    found.forEach((w, i) => (next[at + i] = w));
    return { words: next, focus: Math.min(PHRASE_LENGTH - 1, at + found.length) };
  }

  const phraseReady = (words) => words.length === PHRASE_LENGTH && words.every((w) => w.trim() !== "");

  function memberName(t, m) {
    const app = m.name || m.app || "";
    return m.device ? fill(t.memberOn, { app, device: m.device }) : app;
  }

  // The apps that keep the phrase, those in reach first, each named once. A
  // plugin keeps only its keys.
  function keepers(t, members) {
    const at = members.filter((m) => m.form && m.form !== "plugin");
    const names = [];
    at.filter((m) => m.online)
      .concat(at.filter((m) => !m.online))
      .forEach((m) => {
        const n = memberName(t, m);
        if (names.indexOf(n) < 0) names.push(n);
      });
    return names;
  }

  function changeWhere(t, lang, app, members) {
    const names = keepers(t, members);
    return names.length ? fill(t.changeElsewhere, { apps: listOf(lang, names, "disjunction") }) : fill(t.changeNowhere, { app });
  }

  // The other members as the cards draw them: one card per app, its
  // instances in the order they came.
  function groupApps(members) {
    const apps = [];
    const byId = {};
    members.forEach((m) => {
      const key = m.app || m.name || m.id;
      if (!byId[key]) {
        byId[key] = { id: key, name: m.name || m.app || "", icon: "", members: [] };
        apps.push(byId[key]);
      }
      if (!byId[key].icon && m.icon) byId[key].icon = m.icon;
      byId[key].members.push(m);
    });
    return apps;
  }

  function othersLine(t, lang, members) {
    const apps = groupApps(members).length;
    const devices = new Set(members.map((m) => m.device)).size;
    return fill(t.othersLine, { apps: plural(lang, t.othersApps, apps), devices: plural(lang, t.othersDevices, devices) });
  }

  function seen(lang, at) {
    const d = new Date(at);
    if (!at || isNaN(d.getTime())) return "";
    try {
      return d.toLocaleString(lang, { dateStyle: "medium", timeStyle: "short" });
    } catch (e) {
      return d.toLocaleString();
    }
  }

  function storageKey(app) {
    return "glim-fortress-others-" + app;
  }

  function storedShow(app) {
    try {
      return localStorage.getItem(storageKey(app)) === "1";
    } catch (e) {
      return false;
    }
  }

  function storeShow(app, on) {
    try {
      localStorage.setItem(storageKey(app), on ? "1" : "0");
    } catch (e) {
      // The switch still holds until the page reloads.
    }
  }

  function h(tag, attrs, children) {
    const n = document.createElement(tag);
    Object.keys(attrs || {}).forEach((k) => {
      const v = attrs[k];
      if (v === undefined || v === null || v === false) return;
      if (k === "class") n.className = v;
      else if (k === "text") n.textContent = v;
      else if (k.slice(0, 2) === "on") n.addEventListener(k.slice(2), v);
      else n.setAttribute(k, v === true ? "" : v);
    });
    (children || []).forEach((c) => {
      if (c === null || c === undefined || c === false) return;
      n.appendChild(typeof c === "string" ? document.createTextNode(c) : c);
    });
    return n;
  }

  function glyph(name) {
    const s = h("span", { class: "glim-fc-g", "aria-hidden": "true" });
    s.innerHTML = GLYPHS[name] || "";
    return s;
  }

  function button(label, name, attrs) {
    return h("button", Object.assign({ type: "button", class: "glim-fc-btn" }, attrs), [glyph(name), h("span", { class: "glim-fc-l", text: label })]);
  }

  function info(tip) {
    return h("span", { class: "glim-fc-info", tabindex: "0", role: "img", "aria-label": tip, "data-tip": tip }, [glyph("IconInfo")]);
  }

  function badge(label, tone) {
    return h("span", { class: "glim-fc-badge" + (tone ? " glim-fc-badge-" + tone : ""), text: label });
  }

  function shake(el) {
    if (!el) return;
    el.classList.remove("glim-fc-shake");
    void el.offsetWidth;
    el.classList.add("glim-fc-shake");
  }

  const FOCUSABLE = 'button:not([disabled]), input:not([disabled]), [href], [tabindex]:not([tabindex="-1"])';

  /*
   * A window over the page: its title as a badge on the top edge, a body and
   * a row of buttons. Escape and a click beside it close it unless it waits
   * for an answer, and the focus goes back to the button that opened it, or
   * to the card once that button is gone.
   */
  function openWindow(card, opener, t) {
    const scrim = h("div", { class: "glim-fc-scrim" });
    const title = h("h2", { class: "glim-fc-win-title", id: "glim-fc-win-t" });
    const body = h("div", { class: "glim-fc-win-body" });
    const act = h("div", { class: "glim-fc-win-act" });
    const box = h("div", { class: "glim-fc-win", role: "dialog", "aria-modal": "true", "aria-labelledby": title.id, tabindex: "-1" }, [title, body, act]);
    const wins = h("div", { class: "glim-fc-wins" }, [scrim, box]);
    document.body.appendChild(wins);
    let waiting = false;
    const w = {
      box,
      set waiting(on) {
        waiting = on;
      },
      step(name, nodes, buttons) {
        title.textContent = name;
        body.textContent = "";
        act.textContent = "";
        nodes.forEach((n) => n && body.appendChild(n));
        buttons.forEach((b) => act.appendChild(b));
        const first = body.querySelector("input") || act.querySelector(".glim-fc-key") || act.querySelector("button");
        if (first) first.focus();
        else box.focus();
      },
      close() {
        if (!wins.parentNode) return;
        wins.remove();
        document.removeEventListener("keydown", onKey, true);
        const back = opener && opener.isConnected ? opener : card;
        if (back) back.focus();
        if (w.onclose) w.onclose();
      },
      onclose: null,
    };
    function onKey(e) {
      if (e.key === "Escape" && !waiting) {
        e.preventDefault();
        w.close();
      } else if (e.key === "Tab") {
        const all = Array.prototype.filter.call(box.querySelectorAll(FOCUSABLE), (n) => n.offsetParent !== null);
        if (!all.length) return;
        const first = all[0];
        const last = all[all.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      }
    }
    scrim.addEventListener("click", () => {
      if (!waiting) w.close();
    });
    document.addEventListener("keydown", onKey, true);
    w.cancel = () => button(t.cancel, "IconClose", { class: "glim-fc-btn glim-fc-win-btn", onclick: () => w.close() });
    w.closer = () => button(t.close, "IconClose", { class: "glim-fc-btn glim-fc-win-btn", onclick: () => w.close() });
    return w;
  }

  function errorCode(e) {
    return (e && (e.code || e.error)) || "";
  }

  /**
   * Draws the fortress into `el`. `adapter` answers each step from the app's
   * server: `load()` the state, `join(words)`, `leave()` and `remove(id)` the
   * state after the step, and `status(id)` the report and log of a plugin
   * member, if the app can read them. A state of null hides the card, for an
   * app whose fortress is not running. `opts.lang` is the page's language and
   * `opts.app` the app showing it, `{ id, name }`. `opts.rows` adds rows of
   * the app's own to the card, each `{ label, about, value, button: { label,
   * glyph }, run }`. A state may carry `hints`, lines of the app's own in its
   * own words under the card's heading, each `{ text, tone, action }` with
   * tone `warn` (the default) or `quiet`, and an action `{ label, glyph,
   * join }` that opens the window for the phrase, or `{ label, glyph, run }`.
   */
  function mount(el, adapter, opts) {
    const lang = (opts && opts.lang) || "en";
    const app = (opts && opts.app) || { id: "app", name: "" };
    const extra = (opts && opts.rows) || [];
    const t = textFor(lang);
    let state = null;
    let failed = false;
    let win = null;
    let show = storedShow(app.id);
    let timer = null;
    let gone = false;

    el.classList.add("glim-fc");
    el.setAttribute("lang", lang);

    function load() {
      return Promise.resolve()
        .then(() => adapter.load())
        .then(
          (s) => {
            state = s ? normal(s) : null;
            failed = false;
            draw();
          },
          () => {
            failed = true;
            draw();
          },
        );
    }

    function normal(s) {
      return {
        joined: !!s.joined,
        device: s.device || "",
        removedHere: !!s.removedHere,
        phraseMoved: !!s.phraseChanged,
        members: Array.isArray(s.members) ? s.members : [],
        removed: Array.isArray(s.removed) ? s.removed : [],
        hints: Array.isArray(s.hints) ? s.hints.filter((x) => x && x.text) : [],
      };
    }

    function took(s) {
      if (s) state = normal(s);
      draw();
    }

    function row(label, about, sub, end, id) {
      const lbl = h("div", { class: "glim-fc-lbl" }, [h("span", { class: "glim-fc-name", id, text: label }), about ? info(about) : null]);
      const start = h("div", { class: "glim-fc-start" }, [lbl, sub ? h("p", { class: "glim-fc-sub", text: sub }) : null]);
      return h("div", { class: "glim-fc-row" }, [start, h("div", { class: "glim-fc-ctl" }, end)]);
    }

    function draw() {
      if (gone) return;
      el.textContent = "";
      if (!state && !failed) {
        el.hidden = true;
        return;
      }
      el.hidden = false;
      const card = h("section", { class: "glim-fc-card", "aria-labelledby": "glim-fc-h", "data-fortress": "card" }, [
        h("h3", { class: "glim-fc-h", id: "glim-fc-h", text: t.fortress }),
      ]);
      el.appendChild(card);
      if (failed) {
        card.appendChild(h("p", { class: "glim-fc-sub glim-fc-error", role: "alert", text: t.failed }));
        return;
      }
      const s = state;
      if (s.removedHere && !s.joined) note(card, t.removedHereTitle, fill(t.removedHereHint, { app: app.name }), "removed-here");
      if (s.joined && s.phraseMoved) note(card, t.changedTitle, fill(t.phraseMoved, { app: app.name }), "moved");
      s.hints.forEach((x) => card.appendChild(hint(x)));

      if (!s.joined) {
        card.appendChild(
          row(t.joinTitle, t.about, fill(t.outHint, { app: app.name }), [
            button(t.enterPhrase, "IconSignIn", { class: "glim-fc-btn glim-fc-key", "data-fortress": "open-join", onclick: (e) => openJoin(e.currentTarget) }),
          ]),
        );
        extraRows(card);
        return;
      }

      const phraseEnd = [];
      if (s.phraseMoved) {
        phraseEnd.push(button(t.enterPhrase, "IconSignIn", { class: "glim-fc-btn glim-fc-key", "data-fortress": "open-join", onclick: (e) => openJoin(e.currentTarget) }));
      }
      card.appendChild(row(t.phraseTitle, null, changeWhere(t, lang, app.name, s.members), phraseEnd));
      card.appendChild(
        row(t.leaveRow, fill(t.leaveAbout, { app: app.name }), s.device, [
          button(t.leaveConfirm, "IconSignOut", { "data-fortress": "open-leave", onclick: (e) => openLeave(e.currentTarget) }),
        ]),
      );
      if (s.removed.length) {
        card.appendChild(
          h("div", { class: "glim-fc-quiet", role: "status", "data-fortress": "removed" }, [
            glyph("IconInfo"),
            h("span", { text: fill(t.removedNote, { devices: listOf(lang, s.removed.map((r) => memberName(t, r))) }) + " " + changeWhere(t, lang, app.name, s.members) }),
          ]),
        );
      }
      extraRows(card);
      if (s.members.length) {
        const sw = h("input", { type: "checkbox", class: "glim-fc-sw", role: "switch", "aria-labelledby": "glim-fc-others", "data-fortress": "others" });
        sw.checked = show;
        sw.addEventListener("change", () => {
          show = sw.checked;
          storeShow(app.id, show);
          draw();
          const again = el.querySelector('[data-fortress="others"]');
          if (again) again.focus();
        });
        card.appendChild(row(t.showOthers, t.about, othersLine(t, lang, s.members), [sw], "glim-fc-others"));
      }
      if (show && s.members.length) {
        const grid = h("div", { class: "glim-fc-grid" });
        groupApps(s.members).forEach((a) => grid.appendChild(appCard(a)));
        el.appendChild(grid);
      }
    }

    function hint(x) {
      const line = h("div", { class: x.tone === "quiet" ? "glim-fc-quiet" : "glim-fc-note", role: "status", "data-fortress": "hint" }, [
        glyph("IconInfo"),
        h("span", { class: "glim-fc-hint-text", text: x.text }),
      ]);
      const a = x.action;
      if (a && a.label) {
        const b = button(a.label, a.glyph || (a.join ? "IconSignIn" : "IconExternalLink"), {
          "data-fortress": a.join ? "open-join" : "hint-action",
          onclick: (e) => {
            if (a.join) return openJoin(e.currentTarget);
            b.disabled = true;
            Promise.resolve()
              .then(() => a.run())
              .then(
                () => load(),
                () => {
                  b.disabled = false;
                  shake(b);
                },
              );
          },
        });
        line.appendChild(b);
      }
      return line;
    }

    function note(card, title, text, kind) {
      card.appendChild(
        h("div", { class: "glim-fc-note", role: "status", "data-fortress": kind }, [
          glyph("IconInfo"),
          h("span", {}, [h("strong", { text: title }), " ", text]),
        ]),
      );
    }

    function extraRows(card) {
      extra.forEach((r) => {
        const end = [];
        if (r.button) {
          const b = button(r.button.label, r.button.glyph || "IconExternalLink", {
            onclick: () => {
              b.disabled = true;
              Promise.resolve()
                .then(() => r.run())
                .then(
                  () => (b.disabled = false),
                  () => {
                    b.disabled = false;
                    shake(b);
                  },
                );
            },
          });
          end.push(b);
        }
        card.appendChild(row(r.label, r.about, r.value, end));
      });
    }

    function presence(m) {
      return m.online ? badge(t.connected, "ok") : badge(t.away);
    }

    function facts(m) {
      const away = !m.online && m.lastSeen ? fill(t.lastSeen, { time: seen(lang, m.lastSeen) }) : "";
      return [m.device, m.version ? "v" + m.version : "", away].filter(Boolean).join(" · ");
    }

    function logo(a) {
      const tile = h("span", { class: "glim-fc-logo", "aria-hidden": "true" });
      if (a.icon) tile.appendChild(h("img", { src: a.icon, alt: "" }));
      else tile.appendChild(h("span", { class: "glim-fc-initial", text: (a.name || "?").charAt(0) }));
      return tile;
    }

    function actions(a, m) {
      const out = [];
      // A plugin that is away has no status to show, so it offers no Open.
      if (m.form === "plugin" && m.online && adapter.status) {
        out.push(button(t.open, "IconExternalLink", { "data-fortress": "open", "aria-label": t.open + ": " + memberName(t, m), onclick: (e) => openStatus(e.currentTarget, a, m) }));
      }
      out.push(button(t.remove, "IconTrash", { "data-fortress": "open-remove", "aria-label": t.remove + ": " + memberName(t, m), onclick: (e) => openRemove(e.currentTarget, m) }));
      return out;
    }

    function appCard(a) {
      const many = a.members.length > 1;
      const main = a.members[0];
      const forms = [];
      a.members.forEach((m) => {
        const f = m.form ? t.forms[m.form] || m.form : "";
        if (f && forms.indexOf(f) < 0) forms.push(f);
      });
      const badges = h("span", { class: "glim-fc-badges" }, many ? [] : [main.relay ? badge(t.viaRelay) : null, presence(main)]);
      const body = h("div", { class: "glim-fc-app-body" }, [
        h("div", { class: "glim-fc-app-top" }, [h("h4", { class: "glim-fc-app-name", text: a.name }), badges]),
        forms.length ? h("p", { class: "glim-fc-eyebrow", text: forms.join(", ") }) : null,
        h("p", { class: "glim-fc-facts", text: many ? plural(lang, t.instanceCount, a.members.length) : facts(main) }),
      ]);
      if (many) {
        const list = h("ul", { class: "glim-fc-rows" });
        a.members.forEach((m) => {
          list.appendChild(
            h("li", { class: "glim-fc-inst" }, [
              h("span", { class: "glim-fc-inst-text" }, [h("span", { class: "glim-fc-inst-name", text: m.device }), h("span", { class: "glim-fc-facts", text: facts(m).replace(m.device, "").replace(/^ · /, "") })]),
              h("span", { class: "glim-fc-inst-end" }, [m.relay ? badge(t.viaRelay) : null, presence(m)].concat(actions(a, m))),
            ]),
          );
        });
        body.appendChild(list);
      } else {
        body.appendChild(h("div", { class: "glim-fc-app-act" }, actions(a, main)));
      }
      return h("article", { class: "glim-fc-app", "data-fortress-app": a.id }, [logo(a), body]);
    }

    function fail(w, error, btn) {
      error.textContent = t.failed;
      error.hidden = false;
      shake(btn);
      w.waiting = false;
    }

    function errorLine() {
      return h("p", { class: "glim-fc-win-error", role: "alert", hidden: true });
    }

    function openJoin(opener) {
      win = openWindow(el, opener, t);
      win.onclose = closed;
      let words = new Array(PHRASE_LENGTH).fill("");
      const error = errorLine();
      const go = button(t.join, "IconSignIn", { class: "glim-fc-btn glim-fc-win-btn glim-fc-key", "data-fortress": "join", disabled: true });
      const inputs = [];
      const fields = h("div", { class: "glim-fc-words" });
      for (let i = 0; i < PHRASE_LENGTH; i++) {
        const input = h("input", {
          class: "glim-fc-word",
          "data-word": String(i),
          "aria-label": fill(t.word, { n: i + 1 }),
          autocomplete: "off",
          autocapitalize: "none",
          autocorrect: "off",
          spellcheck: "false",
        });
        input.addEventListener("input", () => {
          const r = enterWords(words, i, input.value);
          words = r.words;
          inputs.forEach((x, k) => {
            if (k !== i || r.focus !== i) x.value = words[k];
          });
          go.disabled = !phraseReady(words);
          error.hidden = true;
          if (r.focus !== i) inputs[r.focus].focus();
        });
        inputs.push(input);
        fields.appendChild(h("label", { class: "glim-fc-field" }, [h("span", { class: "glim-fc-num", text: String(i + 1) }), input]));
      }
      const form = h("form", { class: "glim-fc-win-text" }, [
        h("p", { text: t.about }),
        fields,
        h("p", { class: "glim-fc-win-sub", text: t.phraseWhere }),
        error,
        h("button", { type: "submit", hidden: true }),
      ]);
      function join() {
        if (!phraseReady(words) || go.disabled) return;
        go.disabled = true;
        win.waiting = true;
        error.hidden = true;
        Promise.resolve()
          .then(() => adapter.join(words.map((w) => w.trim().toLowerCase())))
          .then(
            (s) => {
              win.waiting = false;
              took(s);
              joined();
            },
            (e) => {
              const code = errorCode(e);
              if (code === "member") {
                win.waiting = false;
                win.close();
                return;
              }
              go.disabled = false;
              error.textContent = code === "phrase" ? t.joinRefused : t.failed;
              error.hidden = false;
              shake(go);
              win.waiting = false;
            },
          );
      }
      form.addEventListener("submit", (e) => {
        e.preventDefault();
        join();
      });
      go.addEventListener("click", join);
      win.step(t.joinTitle, [form], [win.cancel(), go]);
      win.box.classList.add("glim-fc-win-wide");
    }

    function joined() {
      const mark = h("span", { class: "glim-fc-done", "aria-hidden": "true" }, [glyph("IconCheck")]);
      win.step(t.joinTitle, [h("div", { class: "glim-fc-result" }, [mark, h("p", { class: "glim-fc-result-title", text: fill(t.joinedTitle, { app: app.name }) })])], [win.closer()]);
    }

    function openLeave(opener) {
      win = openWindow(el, opener, t);
      win.onclose = closed;
      const error = errorLine();
      const go = button(t.leaveConfirm, "IconSignOut", { class: "glim-fc-btn glim-fc-win-btn glim-fc-key", "data-fortress": "leave" });
      go.addEventListener("click", () => {
        go.disabled = true;
        win.waiting = true;
        Promise.resolve()
          .then(() => adapter.leave())
          .then(
            (s) => {
              win.waiting = false;
              took(s);
              win.close();
            },
            (e) => {
              if (errorCode(e) === "notMember") {
                win.waiting = false;
                win.close();
                return;
              }
              go.disabled = false;
              fail(win, error, go);
            },
          );
      });
      win.step(
        t.leaveTitle,
        [h("div", { class: "glim-fc-win-text" }, [h("p", { text: fill(t.leaveBody, { app: app.name }) }), h("p", { class: "glim-fc-win-sub", text: t.leaveBack }), error])],
        [win.cancel(), go],
      );
    }

    function openRemove(opener, m) {
      const name = memberName(t, m);
      win = openWindow(el, opener, t);
      win.onclose = closed;
      const error = errorLine();
      const go = button(t.remove, "IconTrash", { class: "glim-fc-btn glim-fc-win-btn glim-fc-key", "data-fortress": "remove" });
      go.addEventListener("click", () => {
        go.disabled = true;
        win.waiting = true;
        Promise.resolve()
          .then(() => adapter.remove(m.id))
          .then(
            (s) => {
              win.waiting = false;
              took(s);
              // The member can still get back in with the phrase, so the
              // window says where it changes.
              win.step(
                t.removedTitle,
                [
                  h("div", { class: "glim-fc-win-text" }, [
                    h("p", { text: fill(t.removedBody, { device: name }) }),
                    h("p", { class: "glim-fc-win-sub", "data-fortress": "change-where", text: changeWhere(t, lang, app.name, state ? state.members : []) }),
                  ]),
                ],
                [win.closer()],
              );
            },
            (e) => {
              if (errorCode(e) === "noMember") {
                win.waiting = false;
                win.close();
                return;
              }
              go.disabled = false;
              fail(win, error, go);
            },
          );
      });
      win.step(
        fill(t.removeTitle, { device: name }),
        [h("div", { class: "glim-fc-win-text" }, [h("p", { text: fill(t.removeBody, { device: name }) }), h("p", { class: "glim-fc-win-sub", text: t.removeBack }), error])],
        [win.cancel(), go],
      );
    }

    const TONES = { off: "", ok: "ok", starting: "active", warning: "warn", overdue: "warn", failed: "fail" };

    function level(severity) {
      if (severity === "warn") return badge(t.levels.warn, "warn");
      if (severity === "error") return badge(t.levels.error, "fail");
      return null;
    }

    function section(title, items) {
      return items.length ? h("section", { class: "glim-fc-status-sec" }, [h("h3", { class: "glim-fc-status-h", text: title }), h("ul", { class: "glim-fc-status-list" }, items)]) : null;
    }

    function openStatus(opener, a, m) {
      win = openWindow(el, opener, t);
      win.onclose = closed;
      const name = memberName(t, m);
      const where = h("p", { class: "glim-fc-win-sub", text: fill(t.statusWhere, { app: a.name, device: m.device }) });
      win.step(name, [h("div", { class: "glim-fc-win-text" }, [h("p", { class: "glim-fc-win-sub", role: "status", text: fill(t.statusLoading, { app: a.name }) }), where])], [win.closer()]);
      win.box.classList.add("glim-fc-win-wide");
      const mine = win;
      Promise.resolve()
        .then(() => adapter.status(m.id))
        .then(
          (r) => {
            if (win !== mine) return;
            const report = (r && r.report) || null;
            const log = ((r && r.log) || []).slice(0, STATUS_LOG_LINES);
            const nodes = [];
            if (report) {
              const known = Object.prototype.hasOwnProperty.call(t.states, report.state);
              nodes.push(h("p", { class: "glim-fc-status-state" }, [badge(known ? t.states[report.state] : report.state, known ? TONES[report.state] : "")]));
              const needs = report.needsYou || [];
              const work = report.work || [];
              const next = report.next || [];
              nodes.push(section(t.statusNeedsYou, needs.map((n) => h("li", {}, [level(n.severity), h("span", { text: n.text })]))));
              nodes.push(
                section(
                  t.statusWork,
                  work.map((w) =>
                    h("li", { class: "glim-fc-work" }, [
                      h("span", { text: w.title }),
                      typeof w.progress === "number" ? h("progress", { class: "glim-fc-progress", max: "1", value: String(w.progress) }) : null,
                    ]),
                  ),
                ),
              );
              nodes.push(section(t.statusNext, next.map((n) => h("li", {}, [h("span", { text: n.title }), h("span", { class: "glim-fc-at", text: n.at })]))));
              if (!needs.length && !work.length && !next.length) nodes.push(h("p", { class: "glim-fc-win-sub", text: t.statusQuiet }));
            } else {
              nodes.push(h("p", { class: "glim-fc-win-error", role: "alert", text: (r && r.error) || t.failed }));
            }
            nodes.push(section(t.statusLog, log.map((l) => h("li", {}, [h("span", { class: "glim-fc-at", text: l.at }), level(l.level), h("span", { text: l.text })]))));
            nodes.push(where);
            win.step(name, [h("div", { class: "glim-fc-win-text glim-fc-status" }, nodes)], [win.closer()]);
          },
          () => {
            if (win !== mine) return;
            win.step(name, [h("div", { class: "glim-fc-win-text" }, [h("p", { class: "glim-fc-win-error", role: "alert", text: t.failed }), where])], [win.closer()]);
          },
        );
    }

    function closed() {
      win = null;
    }

    // The state stays fresh while the page shows, so a member that goes away
    // or comes back changes its badge without a reload.
    function tick() {
      if (!gone && !win && document.visibilityState !== "hidden" && (el.hidden || el.offsetParent !== null)) load();
    }

    load();
    timer = setInterval(tick, POLL_MS);
    return {
      refresh: load,
      destroy() {
        gone = true;
        clearInterval(timer);
        if (win) win.close();
        el.textContent = "";
      },
    };
  }

  root.GlimFortressCard = { mount, text: textFor, fill, enterWords, keepers, groupApps };
})(typeof window !== "undefined" ? window : globalThis);
