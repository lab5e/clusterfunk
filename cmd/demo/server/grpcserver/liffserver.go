package grpcserver

//
//Copyright 2019 Telenor Digital AS
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
import (
	"context"
	"fmt"
	"math/rand"
	"net"

	"github.com/lab5e/clusterfunk/cmd/demo"
	"github.com/lab5e/clusterfunk/pkg/funk"
	"github.com/lab5e/clusterfunk/pkg/funk/sharding"
	"github.com/lab5e/clusterfunk/pkg/serverfunk"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// This is the core demo service. It's a simple gRPC service with a single
// method.

// StartDemoServer starts the demo (gRPC) server.
func StartDemoServer(endpoint string, endpointName string, cluster funk.Cluster, shards sharding.ShardMap, metricsType string) {
	// Set up the local gRPC server.
	liffServer := newLiffServer(cluster.NodeID())

	clientProxy := serverfunk.NewProxyConnections(endpointName, shards, cluster)

	// The dial options takes care of all proxying but you must provide a shard
	// conversion function (see below)
	server := grpc.NewServer(serverfunk.WithClusterFunk(cluster,
		createShardConversionFunc(shards), clientProxy, metricsType)...)

	demo.RegisterDemoServiceServer(server, liffServer)

	listener, err := net.Listen("tcp", endpoint)
	if err != nil {
		log.WithError(err).
			WithField("endpoint", endpoint).
			Panic("Unable to create TCP listener")
	}

	log.WithField("endpoint", endpoint).Info("Lauching gRPC demo server")

	if err := server.Serve(listener); err != nil {
		log.WithError(err).
			WithField("endpoint", endpoint).
			Panic("Unable to launch node management gRPC interface")
	}
}

// The shard conversion function is used by the interceptor and returns two parameters:
// The computed shard (id) for the request and the expected return type. The return type
// must be returned by the internal interceptors so that the type is
func createShardConversionFunc(shardMap sharding.ShardMap) serverfunk.ShardConversionFunc {
	shardFunc := sharding.NewIntSharder(int64(len(shardMap.Shards())))

	return func(request interface{}) (int, interface{}) {
		switch v := request.(type) {
		case *demo.LiffRequest:
			return shardFunc(v.ID), &demo.LiffResponse{}
		}
		panic(fmt.Sprintf("Unknown request type %T", request))
	}
}

type liffServer struct {
	nodeID string
}

func newLiffServer(nodeID string) demo.DemoServiceServer {
	return &liffServer{nodeID: nodeID}
}

func (l *liffServer) Liff(ctx context.Context, req *demo.LiffRequest) (*demo.LiffResponse, error) {
	return &demo.LiffResponse{
		ID:         req.ID,
		Definition: liffs[rand.Intn(len(liffs))],
		NodeID:     l.nodeID,
	}, nil
}

// Copied from http://liff.hivemind.net/ which again have copied them from
// "The Meaning of Liff" and "The Deeper Meaning of Liff" by Douglas Adams.
// If you do not own any works of Douglas Adams now is the time to get one.
var liffs = []string{
	"Aasleagh (n.): A liqueur made only for drinking at the end of a revoltingly long bottle party when all the drinkable drink has been drunk.",
	"Aboyne (vb.): To beat an expert at a game of skill by playing so appallingly that none of his clever tactics or strategies are of any use to him.",
	"Abruzzo (n.): The worn patch of ground under a swing.",
	"Acklins (pl. n.): The odd twinges you get in parts of your body when you scratch other parts.",
	"Ahenny (adj.): The way people stand when examining other people's bookshelves.",
	"Aigburth (n.): Any piece of readily identifiable anatomy found amongst cooked meat.",
	"Aith (n.): The single bristle that sticks out sideways on a cheap paintbrush.",
	"Albacete (n.): A single surprisingly long hair growing in the middle of nowhere.",
	"Alcoy (adj.): Wanting to be bullied into having another drink.",
	"Amlwch (n.): A British Rail sandwich which has been kept soft by being regularly washed and resealed in clingfilm.",
	"Ampus (n.): A lurid bruise which you can't remember getting.",
	"Anantnag (vb.): (Eskimo term) To bang your thumbs between the oars when rowing.",
	"Aasleagh (n.): A liqueur made only for drinking at the end of a revoltingly long bottle party when all the drinkable drink has been drunk.",
	"Aboyne (vb.): To beat an expert at a game of skill by playing so appallingly that none of his clever tactics or strategies are of any use to him.",
	"Abruzzo (n.): The worn patch of ground under a swing.",
	"Acklins (pl. n.): The odd twinges you get in parts of your body when you scratch other parts.",
	"Ahenny (adj.): The way people stand when examining other people's bookshelves.",
	"Aigburth (n.): Any piece of readily identifiable anatomy found amongst cooked meat.",
	"Aith (n.): The single bristle that sticks out sideways on a cheap paintbrush.",
	"Albacete (n.): A single surprisingly long hair growing in the middle of nowhere.",
	"Alcoy (adj.): Wanting to be bullied into having another drink.",
	"Amlwch (n.): A British Rail sandwich which has been kept soft by being regularly washed and resealed in clingfilm.",
	"Ampus (n.): A lurid bruise which you can't remember getting.",
	"Anantnag (vb.): (Eskimo term) To bang your thumbs between the oars when rowing.",
	"Badachonacher (n.): An on-off relationship which never gets resolved.",
	"Balemartine (n.): The look which says, 'Stop talking to that woman at once.'",
	"Bathel (vb.): To pretend to have read the book under discussion when in fact you've only seen the TV series.",
	"Baughurst (n.): That kind of large fierce ugly woman who owns a small fierce ugly dog.",
	"Bauple (n.): An indeterminate pustule which could be either a spot or a bite.",
	"Beaulieu Hill (n.): The optimum vantage point from which to view people undressing in the bedroom across the street.",
	"Belding (n.): The technical name for a stallion after its first ball has been cut off. Any notice which reads 'Beware of the Belding' should be taken very, very seriously.",
	"Belper (n.): A knob of someone else's chewing gum which you unexpectedly find your hand resting on under the passenger seat of your car or on somebody's thigh under their skirt.",
	"Bickerstaffe (n.): The person in an office that everyone whinges about in the pub. Many large corporations deliberately employ bickerstaffes in each department.",
	"Bishop's Caundle (n.): An opening gambit before a game of chess where the missing pieces are replaced by small ornaments from the mantelpiece.",
	"Bodmin (n.): That irrational and inevitable discrepancy between the amount pooled and the amount needed when a large group of people try to pay a bill together after a meal.",
	"Boinka (n.): The noise through the wall which tells you that the people next door enjoy a better sex life than you do.",
	"Boolteens (pl. n.): The small scattering of foreign coins and halfpennies which inhabit dressing tables. Since they are never used and never thrown away boolteens account for a significant drain on the world's money supply.",
	"Boscastle (n.): The huge pyramid of tin cans placed just inside the entrance to a supermarket.",
	"Brindle (vb.): To remember suddenly where it is you're meant to be going after you've already been driving for ten minutes.",
	"Canudos (n.): The desire of married couples to see their single friends pair off.",
	"Clenchwarton (n.): (Archaic) One who assists an exorcist by squeezing whichever part of the possessed the exorcist deems useful.",
	"Climpy (adj.): Allowing yourself to be persuaded to do something and pretending to be reluctant.",
	"Cloates Point (n.): The precise instant at which scrambled eggs are ready.",
	"Clun (n.): A leg which has gone to sleep and has to be hauled around after you.",
	"Clunes (pl. n.): People who just won't go.",
	"Cong (n.): Strange-shaped metal utensil found at the back of the saucepan cupboard. Many authorities believe that congs provide conclusive proof of the exstence of a now extinct form of yellow vegetable which the Victorians used to boil mercilessly.",
	"Coodardy (adj.): Astounded at what you've just managed to get away with.",
	"Cotterstock (n.): A piece of wood used to stir paint and thereafter stored uselessly in the shed in perpetuity.",
	"Craboon (vb.): To shout boisterously from a cliff.",
	"Cromarty (n.): The brittle sludge which clings to the top of ketchup bottles and plastic tomatoes in nasty cafés.",
	"Dalderby (n.): A letter to the editor made meaningless because it refers to a previous letter you didn't read. (See A.H. Hedgehope, July 3rd.)",
	"Dalfibble (vb.): To spend large swathes of your life looking for car keys.",
	"Dalmilling (ptcl. vb.): Continually making small talk to someone who is trying to read a book.",
	"Darvel (vb.): To hold out hope for a better invitation until the last minute.",
	"Deal (n.): The gummy substance found between damp toes.",
	"Dewlish (adj.): (Of the hands and feet.) Prunelike after an overlong bath.",
	"Dinder (vb.): To nod thoughtfully while someone gives you a long and complex set of directions which you know you're never going to remember.",
	"Dipple (vb.): To try to remove a sticky something from one hand with the other, thus causing it to get stuck to the other hand and eventually to anything else you try to remove it with.",
	"Dobwalls (pl. n.): The now hard-boiled bits of nastiness which have to be prised off crockery by hand after it has been through a dishwasher.",
	"Dorchester (n.): Someone else's throaty cough which obscures the crucial part of the rather amusing remark you've just made.",
	"Draffan (n.): An infuriating person who always manages to look much more dashing than anyone else by turning up unshaven and hungover at a formal party.",
	"Duddo (n.): The most deformed potato in any given collection of potatoes.",
	"Dufton (n.): The last page of a document that you always leave face down in the photocopier and have to go and retrieve later.",
	"Duleek (n.): Sudden realization, as you lie in bed waiting for the alarm to go off, that it should have gone off an hour ago.",
	"Dumboyne (n.): The realization that the train you have patiently watched pulling out of the station was the one you were meant to be on.",
	"Dunino (n.): Someone who always wants to do whatever you want to do.",
	"Dunster (n.): A small child hired to bounce at dawn on the occupants of the spare bedroom in order to save on tea and alarm clocks.",
	"Duntish (adj.): Mentally incapacitated by a severe hangover.",
	"Eads (pl. n.): The sludgy bits in the bottom of a dustbin, underneath the actual bin liner.",
	"Eakring (ptcpl. vb.): Wondering what to do next when you've just stormed out of something.",
	"East Wittering (ptcpl. vb.): The same as West Wittering, only it's you they're trying to get away from.",
	"Ely (n.): The first, tiniest inkling that something, somewhere, has gone terribly wrong.",
	"Farnham (n.): The feeling you get at about four o'clock in the afternoon when you haven't got enough done.",
	"Ferfer (n.): One who is very excited that they've had a better  idea than the one you've just suggested.",
	"Finuge (vb.): In any division of foodstuffs equally between several people, to give yourself the extra slice left over.",
	"Fiunary (n.): The safe place you put something and forget where it was.",
	"Fladderbister (n.): That part of a raincoat which trails out of a car after you've closed the door on it.",
	"Foffarty (adj.): Unable to find the right moment to leave.",
	"Foindle (vb.): To queue-jump very discreetly by working one's way up the line without being spotted doing so.",
	"Forsinain (n.): (Archaic) The right of the lord of the manor to molest dwarfs on their birthdays.",
	"Fraddam (n.): The small awkward-shaped piece of cheese which remains after grating a large regular-shaped piece of chesse, and which enables you to grate your fingers.",
	"Framlingham (n.): A kind of burglar alarm in common usage. It is cunningly designed so that it can ring at full volume in the street without apparently disturbing anyone. Other types of framlinghams are burglar alarms fitted to business premises in residential areas, which go off as a matter of regular routine at 5.31 p.m. on a Friday evening and do not get turned off till 9.20 a.m. on Monday morning.",
	"Frating Green (adj.): The shade of green which is supposed to make you feel comfortable in hospitals, industrious in schools and uneasy in police stations.",
	"Fring (n.): The noise made by a lightbulb that has just shone its last.",
	"Fritham (n.): A paragraph that you get stuck on in a book. The more you read it, the less it means to you.",
	"Frolesworth (n.): Measure. The minimum time it is necessary to spend frowning in deep concentration at each picture in an art gallery in order that everyone else doesn't think you're a complete moron.",
	"Gaffney (n.): Someone who deliberately misunderstands things for, he hopes, humorous effect.",
	"Galashiels (pl.n): A form of particularly long sparse sideburns which are part of the mandatory turnout of British Rail guards.",
	"Gammersgill (n.): Embarrassed stammer you emit when a voice answers the phone and you realise that you haven't the faintest recollection of who it is you've just rung.",
	"Garrow (n.): Narrow wiggly furrow left after pulling a hair off a painted surface.",
	"Gartness (n.): The ability to say 'No, there's absolutely nothing the matter, what could possibly be the matter? And anyway I don't want to discuss it,' without moving your lips.",
	"Ghent (adj.): Descriptive of the mood indicated by cartoonists by drawing a character's mouth as a wavy line.",
	"Gignog (n.): Someone who, through the injudicious application of alcohol, is now a great deal less funny than he thinks he is.",
	"Gildersome (adj.): Descriptive of a joke someone tells you which starts well, but which becomes so embellished in the telling that you start to weary of it after scarcely half an hour.",
	"Gilgit (n.): Hidden sharply pointed object which stabs you in the cuticle when you reach into a small pot.",
	"Gilling (n.): The warm tingling you get in your feet when having a really good widdle.",
	"Gipping (ptcpl.vb.): The fish-like opening and closing of the jaws seen amongst people who have recently been to the dentist and are puzzled as to whether their teeth have been put back the right way up.",
	"Golant (adj.): Blank, sly and faintly embarrassed. Pertaining to the expression seen on the face of someone who has clearly forgotten your name.",
	"Gonnabarn (n.): An afternoon wasted on watching an old movie on TV.",
	"Goole (n.): The puddle on the bar into which the barman puts your change.",
	"Greeley (n.): Someone who continually annoys you by continually apologizing for annoying you.",
	"Gress (vb.): (Rare) To stick to the point during a family argument.",
	"Gribun (n.): The person in a crisis who can always be relied on to make a good anecdote out of it.",
	"Gruids (n.): The only bits of an animal left after even the people who make sausage rolls have been at it.",
	"Gulberwick (n.): The small particle that you always think you've got stuck at the back of your throat after you've been sick.",
	"Hagnaby (n.): Someone who looked a lot more attractive in the disco than they do in your bed the next morning.",
	"Harlosh (vb.): To redistribute the hot water in a bath.",
	"Hepple (vb.): To sculpt the contents of a sugar bowl.",
	"Hever (n.): The panic caused by half-hearing the Tannoy in an airport.",
	"High Limerigg (n.): The topmost tread of a staircase which disappears when you're climbing the stairs in darkness.",
	"Hobarris (n.): (Medical) A sperm which carries a high risk of becoming a bank manager.",
	"Hosmer (vb.): (Of a TV newsreader) To continue to stare impassively into the camera when it should have already switched to the sports report.",
	"Hotagen (n.): The aggressiveness with which a shop assistant sells you any piece of high technology which they don't understand themselves.",
	"Hove (adj.): Descriptive of the expression on the face of a person in the presence of another who clearly isn't going to stop talking for a very long time.",
	"Huna (n.): The result of coming to the wrong decision.",
	"Imber (vb.): To lean from side to side while watching a car chase in the cinema.",
	"Jeffers (pl. n.): Persons who honestly believe that a business lunch is going to achieve something.",
	"Jofane (adj.): In breach of the laws of joke telling, e.g. giving away the punchline in advance.",
	"Kabwum (n.): The cutesy humming noise you make as you go to kiss someone on the cheek.",
	"Kent (adj.): Politely determined not to help despite a violent urge to the contrary. Kent expressions are seen on the faces of people who are good at something watching someone else who can't do it at all.",
	"Kalami (n.): The ancient Eastern art of being able to fold road maps properly.",
	"Keele (n.): The horrible smell caused by washing ashtrays.",
	"Kelling (ptcpl. vb.): The action of looking for something all over again in the places you've already looked.",
	"Kettleness (adj.): The quality of not being able to pee while being watched.",
	"Kirby (n.): Small but repulsive piece of food prominently attached to a person's face or clothing.",
	"Lampeter (n.): The fifth member of a foursome.",
	"Lemvig (n.): A person who can be relied upon to be doing worse than you.",
	"Liniclate (adj.): All stiff and achey in the morning and trying to remember why.",
	"Lulworth (n.): Measure of conversation. A lulworth defines the amount of the length, loudness and embarrassment of a statement you make when everyone else in the room unaccountably stops talking at the same moment.",
	"Macroy (n.): An authoritative, confident opinion based on one you read in a newspaper.",
	"Millinocket (n.): The thing that rattles around inside an aerosol can.",
	"Mimbridge (n.): That which two very boring people have in common which enables you to get away from them.",
	"Motspur (n.): The fourth wheel of a supermarket trolley which looks identical to the other three but renders the trolley completely uncontrollable.",
	"Mugeary (n.): (Medical) The substance from which the unpleasant little yellow globules in the corners of a sleepy person's eyes are made.",
	"Nad (n.): Measure defined as the distance between a driver's out-stretched fingertips and the ticket machine in an automatic car-park. 1 nad = 18.4 cm.",
	"Namber (vb.): To hang around the table being too shy to sit next to the person you really want to.",
	"Nantucket (n.): The secret pocket which eats your train ticket.",
	"Naugatuck (n.): A plastic sachet containing shampoo, polyfilla, etc., which it is impossible to open except by biting off the corners.",
	"Nindigully (n.): One who constantly needs to be re-persuaded of something they've already agreed to.",
	"Noak Hoak (n.): A driver who indicated left and turns right.",
	"Nubbock (n.): The kind of person who has to leave before a party can relax and enjoy itself.",
	"Nupend (n.): The amount of small change found in the lining of an old jacket which just saves your bacon.",
	"Oughterby (n.): Someone you don't want to invite to a party but whom you know you have to as a matter of duty.",
	"Ozark (n.): One who offers to help after all the work has been done.",
	"Papple (vb.): To do what babies do to soup with their spoons.",
	"Pelutho (n.): A South American ball game. The balls are whacked against a brick wall with a stout wooden bat until the prisoner confesses.",
	"Perranzabuloe (n.): One of those spray things used to wet ironing with.",
	"Peterculter (n.): Someone you don't want to be friends with who rings you up at eight-monthly intervals and suggests you get together soon.",
	"Plumgarths (pl.n.): The corrugations on the ankles caused by wearing tight socks.",
	"Plymouth (vb.): To relate an amusing story to someone without remembering that it was they who told it to you in the first place.",
	"Poges (pl.n.): The lumps of dry powder that remain after cooking a packet of soup.",
	"Polyphant (n.): The mythical beast -- part bird, part snake, part jam stain -- which invariably wins children's painting competitions in the 5-7 age group.",
	"Potarch (n.): The eldest male in a soap opera family.",
	"Quenby (n.): A stubborn spot on a window which you spend twenty minutes trying to clean off before discovering it's on the other side of the glass.",
	"Ravenna (n.): Poetic term for the cleavage in a workman's bottom that peeks above the top of his trousers.",
	"Rhymney (n.): That part of a song lyric which you suddenly discover you've been mishearing for years.",
	"Rimbey (n.): The particularly impressive throw of a frisbee which causes it to be lost.",
	"Risplith (n.): The burst of applause which greets the sound of a plate smashing in a canteen.",
	"Rochester (n.): One who is able to gain occupation of the armrests on both sides of their cinema or aircraft seat.",
	"Royston (n.): The man behind you in church who sings with terrific gusto almost three-quarters of a tone off the note.",
	"Rudge (n.): An unjust criticism of your ex-girlfriend's new boyfriend.",
	"Salween (n.): A faint taste of washing-up liquid in a cup of tea.",
	"Satterthwaite (vb.): To spray the person you are talking to with half-chewed breadcrumbs or small pieces of whitebait.",
	"Saucillo (n.): A joke told my someone who completely misjudges the temperament of the person to whom it is told.",
	"Sconser (n.): A person who looks around them when talking to you, to see if there's anyone more interesting about.",
	"Scosthrop (vb.): To make vague opening or cutting movements with the hands when wandering about looking for a tin opener, scissors, etc., in the hope that this will help in some way.",
	"Scraptoft (n.): The absurd flap of hair a vain and balding man grows long above one ear to comb it plastered over the top of his head to the other ear.",
	"Scronkey (n.): Something that hits the window as a result of a violent sneeze.",
	"Sheepy Magna (n.): One who emerges unexpectedly from the wrong bedroom in the morning.",
	"Shimpling (ptcpl. vb.): Lying about the state of your life in order to cheer up your parents.",
	"Shirmers (pl. n.): Tall young men who stand around smiling at weddings as if to suggest that they know the bride rather well.",
	"Sidcup (n.): A hat made from tying knots in the corners of a handkerchief.",
	"Sigglesthorne (n.): Anything used in lieu of a toothpick.",
	"Silloth (n.): Something that was sticky, and is now furry, found on the carpet under the sofa on the morning after a party.",
	"Skagway (n.): Sudden outbreak of cones on a motorway.",
	"Skibbereen (n.): The noise made by a sunburned thigh leaving a plastic chair.",
	"Slubbery (n.): The gooey drips of wax that dribble down the sides of a candle.",
	"Slumbay (n.): The cigarette end someone discovers in the mouthful of lager they have just swigged from a can at the end of a party.",
	"Sneem (n.): Particular kind of frozen smile bestowed on a small child by a parent in mixed company when question, 'Mummy, what's this?' appear to require the answer, 'Er... it's a rubber johnny, darling.'",
	"Soller (vb.): To break something in two while testing if you glued it together properly.",
	"Sompting (n.): The practice of dribbling involuntarily into one's own pillow.",
	"Spreakley (adj.): Irritatingly cheerful in the morning.",
	"Spurger (n.): One who in answer to the question 'How are you?' actually tells you.",
	"Stibbard (n.): The invisible brake pedal on the passenger's side of the car.",
	"Stoke Poges (n.): The tapping movements of an index finger on glass made by a person futilely attempting to communicate with either a tropical fish or a Post Office clerk.",
	"Stody (n.): A small drink which someone nurses for hours so they can stay in the pub.",
	"Stowting (ptcpl. vb.): Feeling a pregnant woman's tummy.",
	"Strelley (n.): Long strip of paper or tape which has got tangled round the wheel of something.",
	"Sturry (n.): A token run. Pedestrians who have chosen to cross a road immediately in front of an approaching vehicle generally give a little wave and break into a sturry. This gives the impression of hurrying without having any practical effect on their speed whatsoever.",
	"Stutton (n.): Tiny melted plastic nodule which fails to help fasten a duvet cover.",
	"Suckley Knowl (n.): A plumber's assistant who never knows where the actual plumber is.",
	"Surby (adj.): Insolently polite, as of policemen who have stopped a motorist.",
	"Sutton and Cheam (ns.): Sutton and Cheam are the two kinds of dirt into which all dirt is divided. 'Sutton' is the dark sort that always gets on to light-coloured things, and 'cheam' the light-coloured sort that always clings on to dark items. Anyone who has ever found Marmite stains on a dress-shirt, or seagull goo on a dinner jacket a) knows all about sutton and cheam, and b) is going to some very curious dinner parties.",
	"Swaffham Bulbeck (n.): An entire picnic lunchtime spent fighting off wasps.",
	"Tidpit (n.): The corner of a toenail from which satisfying little black spots may be sprung.",
	"Timble (vb.): (Of small nasty children) To fall over very gently, look around to see who's about, and then yell blue murder.",
	"Tonypandy (n.): The voice used by presenters on children's television programmes.",
	"Tooting Bec (n.): A car behind which one draws up at the traffic lights and hoots at when the lights go green before realising that the car is parked and there is no one inside.",
	"Trunch (n.): Instinctive resentment of people younger than you.",
	"Tumby (n.): The involuntary abdominal gurgling which fills the silence following someone else's intimate personal revelation.",
	"Urchfont (n.): Sudden stab of hypocrisy which goes through the mind when taking vows as a godparent.",
	"Wawne (n.): A badly supressed yawn.",
	"West Wittering (ptcpl. vb.): The uncontrollable twitching which breaks out when you're trying to get away from the most boring person at a party.",
	"Whasset (n.): A business card in your wallet belonging to someone whom you have no recollection of meeting.",
	"Wigan (n.): If, when talking to someone you know only has one leg, you're trying to treat them perfectly casually and normally, but find to your horror that your conversation is liberally studded with references to (a) Long John Silver, (b) Hopalong Cassidy, (c) the Hokey Cokey, (d) 'putting your foot in it', (e) 'the last leg of the UEFA competition', you are said to have committed a wigan.",
	"Willimantic (adj.): Of a person whose heart is in the wrong place (i.e. between their legs).",
	"Woking (ptcpl. vb.): Standing in the kitchen wondering what you came in here for.",
	"Worksop (n.): A person who never actually gets round to doing anything because he spends all his time writing out lists headed 'Things To Do (Urgent)'.",
	"Yesnaby (n.): A 'yes, maybe' which means 'no'.",
	"Zagreb (n.): A stranger who suddenly clutches an intimate part of your body and then pretends they did it to prevent themselves falling.",
}
