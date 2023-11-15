const cron = require('node-cron')
const { App } = require('@slack/bolt')
const k8s = require('@kubernetes/client-node')

// instantiating slack and k8s apis
const slackApp = new App({
  token: process.env.botsecret,
  appToken: process.env.appsecret,
  socketMode: true
})

const kc = new k8s.KubeConfig()
kc.loadFromCluster()
//console.log(`Current-Context: ${kc.currentContext} \n Contexts ${kc.contexts.} \n Clusters: ${kc.clusters}`)

const k8sApi = kc.makeApiClient(k8s.CoreV1Api)
/***
 * Takes the cronTime provided from the pod and uses that to schedule a job that will
 * query the k8s api for any pods that live longer than the age threshold
 */
async function startCronJob(cronTime, ageThreshold, channel, namespace) {
  cron.schedule(cronTime, async () => {
    pods = await podNeedsRestart(ageThreshold, namespace)
    if (pods.length > 0) {
      notify(pods, channel, ageThreshold)
    }
  }, {
    timezone: "America/Los_Angeles"
  })
}

/**
 * Queries the k8s api for the age of all pods in the given namespace
 * and returns a list of all those that need to be restarted
 */
async function podNeedsRestart(ageThreshold, namespace) {
  let oldPods = [] // will hold the pods that are older than ageThreshold
  const ageThreshMilli = ageThreshold * 60000 // age threshold in milliseconds for comparision
  console.log(`AgeThresholdMilli calculated as ${ageThreshMilli}`)

  // ISSUE - It almost looks like our app is freezing here silently. The next two console logs are never printing.
  var allPods
  // hardcoding namespace 
  try {

    allPods = await k8sApi.listNamespacedPod(namespace) // getting all pods in namespace
  } catch (err) {
    console.log(`Error: ${JSON.stringify(err)}`)
  }

  // checking if pods are older than threshold
  for (let i = 0; i < allPods.body.items.length; i++) {
    let age = Date.now() - allPods.body.items.at(i).status.startTime
    if (ageThreshMilli < age) {
      oldPods.push(allPods.body.items.at(i).metadata.name)
    }
    //console.log(`Age calculated as ${age} milliseconds`)
  }

  // returning all pods that are older than 
  return oldPods
}

/**
 * Takes a list of all pods that need to be restarted and sends a message to
 * specified channel with that list
 */
async function notify(pods, channel, ageThreshold) {
  try {
    await slackApp.client.chat.postMessage({
      channel: channel,
      text: `The following pods are older than your specified ageThreshold of ${ageThreshold} hours and need to be restarted: ${pods}`
    })
  } catch (err) {
    console.log(`Error notifying channel: ${err}`)
    return null
  }
}

// starting cron jobs and slack app
startCronJob(process.env.QUERYTIME, process.env.AGETHRESHOLD, process.env.SLACKCHANNEL, process.env.NAMESPACE)

slackApp.start(process.env.PORT || 3000).then(
  console.log('⚡️ Bolt app is currently running!')
)